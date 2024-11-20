package oncall

// Show oncall information using PagerDuty's API

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"path"
	"strings"
	"text/template"
	"time"

	// this will register the sqlite3 driver

	"github.com/PagerDuty/go-pagerduty"
	"github.com/insomniacslk/hours"
	_ "github.com/mattn/go-sqlite3"
	"github.com/mitchellh/go-homedir"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"gopkg.in/yaml.v2"

	"github.com/insomniacslk/slackbot/pkg/actions"
	"github.com/insomniacslk/slackbot/pkg/credentials"
	"github.com/insomniacslk/slackbot/plugins"
)

func init() {
	if err := plugins.Register("oncall", &Oncall{}); err != nil {
		log.Printf("Failed to register plugin 'oncall': %v", err)
	}
}

type oncallConfig struct {
	DefaultScheduleID string   `yaml:"default_schedule_id"`
	Locations         []string `yaml:"locations"`
	HandoffReminders  struct {
		Enabled      bool   `yaml:"enabled"`
		TemplatePath string `yaml:"template_path"`
		ChannelID    string `yaml:"channel_id"`
		When         []struct {
			Time     string `yaml:"time"`
			Location string `yaml:"location"`
		} `yaml:"when"`
	} `yaml:"handoff_reminders"`
}

// ErrUsage means that the specified command usage is invalid.
var ErrUsage = errors.New("invalid usage")

// Oncall is a PagerDuty oncall plugin.
type Oncall struct {
	Config *oncallConfig
}

// Name returns the plugin name
func (g Oncall) Name() string {
	return "oncall"
}

// Handles returns true if it can handle that command.
func (g Oncall) Handles(cmd string) bool {
	return cmd == "oncall"
}

type reminder struct {
	location *time.Location
	hour     int
	minute   int
	template *template.Template
}

func (r *reminder) String() string {
	return fmt.Sprintf("%d:%02d %s", r.hour, r.minute, r.location)
}

// Load loads the passed configuration.
func (g *Oncall) Load(configYAML []byte) error {
	var conf oncallConfig
	if err := yaml.Unmarshal(configYAML, &conf); err != nil {
		return err
	}
	// expand handoff reminder's template path
	f, err := homedir.Expand(conf.HandoffReminders.TemplatePath)
	if err != nil {
		return fmt.Errorf("failed to expand template_path: %w", err)
	}
	conf.HandoffReminders.TemplatePath = f
	// validate handoff reminder's template, only if reminders are enabled
	reminders := make([]reminder, 0)
	if conf.HandoffReminders.Enabled {
		tmpl, err := template.New(path.Base(conf.HandoffReminders.TemplatePath)).ParseFiles(conf.HandoffReminders.TemplatePath)
		if err != nil {
			return fmt.Errorf("failed to parse handoff_reminder template: %w", err)
		}
		// validate handoff reminder's times
		// TODO identify and remove duplicates
		for _, when := range conf.HandoffReminders.When {
			loc, err := time.LoadLocation(when.Location)
			if err != nil {
				return fmt.Errorf("failed to load location %q: %w", when.Location, err)
			}
			h, err := hours.Parse(when.Time)
			if err != nil {
				return err
			}
			r := reminder{
				template: tmpl,
				location: loc,
				hour:     h.Hour,
				minute:   h.Minute,
			}
			reminders = append(reminders, r)
		}
	}
	if conf.HandoffReminders.Enabled {
		if len(reminders) == 0 {
			return fmt.Errorf("reminders enabled but no reminder is set")
		}
		log.Printf("Oncall reminders enabled\n")
		go g.runReminders(reminders, conf.HandoffReminders.ChannelID)
	} else {
		log.Printf("Oncall reminders not enabled")
	}
	g.Config = &conf
	return nil
}

func updateTimer(reminders []reminder) (*reminder, *time.Time, *time.Timer) {
	now := time.Now()
	var (
		earliest *time.Time
		reminder *reminder
	)
	for _, r := range reminders {
		rtime := time.Date(now.Year(), now.Month(), now.Day(), r.hour, r.minute, 0, 0, r.location)
		if rtime.Before(now) {
			// next one is one day after
			rtime = rtime.Add(24 * time.Hour)
		}
		if earliest == nil {
			earliest = &rtime
			reminder = &r
		} else {
			if rtime.Before(*earliest) {
				earliest = &rtime
				reminder = &r
			}
		}
	}
	return reminder, earliest, time.NewTimer(time.Until(*earliest))
}

func (g *Oncall) runReminders(reminders []reminder, dest string) {
	log.Printf("Running %d oncall reminders", len(reminders))
	for _, r := range reminders {
		log.Printf("- %s", r.String())
	}
	api := slack.New(credentials.SlackBotToken, slack.OptionAppLevelToken(credentials.SlackAppLevelToken))
	client := socketmode.New(api)
	reminder, when, timer := updateTimer(reminders)
	for {
		log.Printf("Next tick: %s", when)
		<-timer.C
		// print reminder
		var out bytes.Buffer
		if err := reminder.template.Execute(&out, nil); err != nil {
			log.Printf("Error: failed to execute oncall reminder template: %v", err)
		}
		reminder, when, timer = updateTimer(reminders)
		actions.Say(client, dest, "", out.String())
	}
}

func (g *Oncall) get(scheduleID string) ([]pagerduty.OnCall, error) {
	client := pagerduty.NewClient(credentials.PagerDutyAPIKey)
	ctx := context.Background()
	opts := pagerduty.ListOnCallOptions{
		ScheduleIDs: []string{scheduleID},
		Includes:    []string{"users"},
		Until:       time.Now().Add(24 * time.Hour).Format(time.RFC3339),
	}
	resp, err := client.ListOnCallsWithContext(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get oncalls: %v", err)
	}
	return resp.OnCalls, nil
}

func timeInLocation(t time.Time, loc *time.Location) string {
	return t.In(loc).Format("Jan 02 15:04 MST")
}

// HandleCmd is called when a .wea/.weather command is invoked.
func (g *Oncall) HandleCmd(client *socketmode.Client, ev *slackevents.MessageEvent, arg string) error {
	var scheduleIDs []string
	locations := make([]*time.Location, 0)
	for _, locName := range g.Config.Locations {
		loc, err := time.LoadLocation(locName)
		if err != nil {
			return fmt.Errorf("failed to load location %q: %w", locName, err)
		}
		locations = append(locations, loc)
	}
	if len(locations) == 0 {
		locations = []*time.Location{time.UTC}
	}
	if arg == "" {
		scheduleIDs = []string{g.Config.DefaultScheduleID}
	} else {
		// search schedules by name
		pdclient := pagerduty.NewClient(credentials.PagerDutyAPIKey)
		ctx := context.Background()
		opts := pagerduty.ListSchedulesOptions{
			Query: arg,
		}
		resp, err := pdclient.ListSchedulesWithContext(ctx, opts)
		if err != nil {
			return fmt.Errorf("schedule search failed: %w", err)
		}
		for _, sc := range resp.Schedules {
			scheduleIDs = append(scheduleIDs, sc.ID)
		}
	}
	if scheduleIDs == nil {
		return fmt.Errorf("invalid empty schedule ID")
	}
	log.Printf("Getting oncalls for schedule IDs %v", scheduleIDs)
	for _, scheduleID := range scheduleIDs {
		oncalls, err := g.get(scheduleID)
		if err != nil {
			return err
		}
		oncallByRotation := make(map[string][]pagerduty.OnCall)
		for _, oncall := range oncalls {
			oncallByRotation[oncall.Schedule.Summary] = append(oncallByRotation[oncall.Schedule.Summary], oncall)
		}
		for sched, oncalls := range oncallByRotation {
			var msg string
			if len(oncalls) > 0 {
				// assume that the schedule URL is the same for all the other
				// items, since they were grouped together by schedule name.
				msg += fmt.Sprintf("*<%s|%s>*", oncalls[0].Schedule.HTMLURL, sched)
			} else {
				msg += "*" + sched + "*"
			}
			idx := 0
			for _, oncall := range oncalls {
				timeFormat := "2006-01-02T15:04:05Z"
				oncallEnd, err := time.Parse(timeFormat, oncall.End)
				if err != nil {
					return fmt.Errorf("failed to parse time %q with format %q: %w", oncall.End, timeFormat, err)
				}
				timeList := make([]string, 0, len(locations))
				for _, loc := range locations {
					timeList = append(timeList, timeInLocation(oncallEnd, loc))
				}
				switch idx {
				case 0:
					msg += fmt.Sprintf(": Current oncall: <%s|%s> (until %s).\n", oncall.User.HTMLURL, oncall.User.Summary, strings.Join(timeList, " | "))
				case 1:
					msg += fmt.Sprintf(" Next: <%s|%s> (until %s)\n", oncall.User.HTMLURL, oncall.User.Summary, strings.Join(timeList, " | "))
				default:
					msg += fmt.Sprintf("       <%s|%s> (until %s)\n", oncall.User.HTMLURL, oncall.User.Summary, strings.Join(timeList, " | "))
				}
				idx++
			}
			threadTS := ""
			if ev.ThreadTimeStamp != "" {
				threadTS = ev.ThreadTimeStamp
			}
			actions.Say(client, ev.Channel, threadTS, msg)
		}
	}
	return nil
}
