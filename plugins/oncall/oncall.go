package oncall

// Show oncall information using PagerDuty's API

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	// this will register the sqlite3 driver

	"github.com/PagerDuty/go-pagerduty"
	_ "github.com/mattn/go-sqlite3"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"

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
	DefaultScheduleID string `json:"default_schedule_id"`
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

// Load loads the passed configuration.
func (g *Oncall) Load(configJSON []byte) error {
	var conf oncallConfig
	if err := json.Unmarshal(configJSON, &conf); err != nil {
		return err
	}
	g.Config = &conf
	return nil
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

// HandleCmd is called when a .wea/.weather command is invoked.
func (g *Oncall) HandleCmd(client *socketmode.Client, ev *slackevents.MessageEvent, arg string) error {
	var scheduleIDs []string
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
				switch idx {
				case 0:
					msg += fmt.Sprintf(": Current oncall: <%s|%s>.", oncall.User.HTMLURL, oncall.User.Summary)
				case 1:
					msg += fmt.Sprintf(" Next: <%s|%s>", oncall.User.HTMLURL, oncall.User.Summary)
				default:
					msg += fmt.Sprintf(", <%s|%s>", oncall.User.HTMLURL, oncall.User.Summary)
				}
				idx++
			}
			actions.Say(client, ev.Channel, msg)
		}
	}
	return nil
}
