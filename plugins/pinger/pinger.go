package pinger

// Ping the oncall or an entire team, trying to match PagerDuty users to Slack users.

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	// this will register the sqlite3 driver

	"github.com/PagerDuty/go-pagerduty"
	_ "github.com/mattn/go-sqlite3"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"gopkg.in/yaml.v2"

	"github.com/insomniacslk/slackbot/pkg/actions"
	"github.com/insomniacslk/slackbot/pkg/credentials"
	"github.com/insomniacslk/slackbot/plugins"
)

func init() {
	if err := plugins.Register("pinger", &Pinger{}); err != nil {
		log.Printf("Failed to register plugin 'pinger': %v", err)
	}
}

type pingerConfig struct {
	ScheduleID    string   `yaml:"schedule_id"`
	FallbackUsers []string `yaml:"fallback_users"`
}

// ErrUsage means that the specified command usage is invalid.
var ErrUsage = errors.New("invalid usage")

// Pinger is a plugin that pings the oncall or an entire team, trying to match the pagerduty oncall to a Slack user.
type Pinger struct {
	Config *pingerConfig
}

// Name returns the plugin name
func (g Pinger) Name() string {
	return "pinger"
}

// Handles returns true if it can handle that command.
func (g Pinger) Handles(cmd string) bool {
	return cmd == "ping"
}

// Load loads the passed configuration.
func (g *Pinger) Load(configYAML []byte) error {
	var conf pingerConfig
	if err := yaml.Unmarshal(configYAML, &conf); err != nil {
		return err
	}
	g.Config = &conf
	return nil
}

func (g *Pinger) getOncalls(scheduleID string) ([]pagerduty.OnCall, error) {
	client := pagerduty.NewClient(credentials.PagerDutyAPIKey)
	ctx := context.Background()
	opts := pagerduty.ListOnCallOptions{
		ScheduleIDs: []string{scheduleID},
		Includes:    []string{"users"},
		Until:       time.Now().Add(24 * time.Hour).Format(time.RFC3339),
	}
	resp, err := client.ListOnCallsWithContext(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get oncalls for schedule ID %q: %v", scheduleID, err)
	}
	return resp.OnCalls, nil
}

// HandleCmd is called when a .wea/.weather command is invoked.
func (g *Pinger) HandleCmd(client *socketmode.Client, ev *slackevents.MessageEvent, arg string) error {
	// ignore `arg`, we only use the configuration file.
	scheduleID := g.Config.ScheduleID
	if scheduleID == "" {
		return fmt.Errorf("`schedule_id` is empty or not set")
	}
	oncalls, err := g.getOncalls(scheduleID)
	log.Printf("Getting oncalls for schedule IDs %v", scheduleID)
	if err != nil {
		return fmt.Errorf("failed to get oncalls for schedule ID %s: %w", scheduleID, err)
	}
	oncallByRotation := make(map[string][]pagerduty.OnCall)
	for _, oncall := range oncalls {
		oncallByRotation[oncall.Schedule.Summary] = append(oncallByRotation[oncall.Schedule.Summary], oncall)
	}
	for sched, oncalls := range oncallByRotation {
		msg := "Ping oncall for "
		if len(oncalls) > 0 {
			// assume that the schedule URL is the same for all the other
			// items, since they were grouped together by schedule name.
			msg += fmt.Sprintf("*<%s|%s>*: ", oncalls[0].Schedule.HTMLURL, sched)
		} else {
			msg += "*" + sched + "*"
		}
		if len(oncalls) == 0 {
			return fmt.Errorf("oncall not found for schedule ID %s", scheduleID)
		}
		oncall := oncalls[0]
		// search Slack user by email, using the oncall's email from PagerDuty
		user, err := client.GetUserByEmail(oncall.User.Email)
		if err != nil {
			log.Printf("Warning: no Slack user found for e-mail %q: %v", oncall.User.Email, err)
			for _, uid := range g.Config.FallbackUsers {
				msg += fmt.Sprintf("<@%s> ", uid)
			}
		} else {
			msg += fmt.Sprintf("<@%s>", user.ID)
		}
		threadTS := ""
		if ev.ThreadTimeStamp != "" {
			threadTS = ev.ThreadTimeStamp
		}
		actions.Say(client, ev.Channel, threadTS, msg)
	}
	return nil
}
