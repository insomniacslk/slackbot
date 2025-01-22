package bot

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/insomniacslk/slackbot/pkg/credentials"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

func New(c *Config) *Bot {
	return &Bot{
		Name:   c.BotName,
		Config: c,
	}
}

// Bot is the main bot object.
type Bot struct {
	Log    *log.Logger
	Name   string
	Config *Config
}

func (b Bot) isCmd(cmd string) bool {
	return strings.HasPrefix(cmd, b.Config.CmdPrefix) && len(cmd)-len(b.Config.CmdPrefix) > 0
}

// Start starts the bot.
func (b *Bot) Start() error {
	logger := log.New(
		os.Stdout,
		fmt.Sprintf("%s: ", b.Name),
		log.Lshortfile|log.LstdFlags,
	)
	if b.Config.LogFile != "" {
		fd, err := os.OpenFile(b.Config.LogFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
		if err != nil {
			return fmt.Errorf("Failed to open log file `%s`: %v", b.Config.LogFile, err)
		}
		log.SetOutput(fd)
		fmt.Printf("Logging to file %s\n", b.Config.LogFile)
	}
	b.Log = logger

	b.Log.Printf("Config: %+v", b.Config)
	api := slack.New(credentials.SlackBotToken, slack.OptionDebug(b.Config.Debug), slack.OptionLog(b.Log), slack.OptionAppLevelToken(credentials.SlackAppLevelToken))
	client := socketmode.New(api, socketmode.OptionDebug(b.Config.Debug), socketmode.OptionLog(b.Log))
	b.Log.Printf("Client created")

	go func() {
		for ev := range client.Events {
			switch ev.Type {
			case socketmode.EventTypeConnecting:
				fmt.Println("Connecting to Slack with Socket Mode...")
			case socketmode.EventTypeConnectionError:
				fmt.Println("Connection failed. Retrying later...")
			case socketmode.EventTypeConnected:
				fmt.Println("Connected to Slack with Socket Mode.")
			case socketmode.EventTypeEventsAPI:
				eventsAPIEvent, ok := ev.Data.(slackevents.EventsAPIEvent)
				if !ok {
					fmt.Printf("Ignored %+v\n", ev)
					continue
				}
				fmt.Printf("Event received: %T %v %+v\n", eventsAPIEvent, eventsAPIEvent.Type, eventsAPIEvent)
				client.Ack(*ev.Request)
				switch eventsAPIEvent.Type {
				case slackevents.CallbackEvent:
					innerEvent := eventsAPIEvent.InnerEvent
					switch iev := innerEvent.Data.(type) {
					case *slackevents.AppMentionEvent:
						_, _, err := client.PostMessage(iev.Channel, slack.MsgOptionText("Yes, hello.", false))
						if err != nil {
							fmt.Printf("failed posting message: %v\n", err)
						}
						fmt.Printf("Posted reply to %v", iev.Channel)
					case *slackevents.MemberJoinedChannelEvent:
						fmt.Printf("user %q joined to channel %q\n", iev.User, iev.Channel)
					case *slackevents.MessageEvent:
						parts := strings.SplitN(iev.Text, " ", 2)
						if len(parts) == 0 {
							// blank line?
							continue
						}
						if !b.isCmd(parts[0]) {
							continue
						}
						cmd := parts[0][len(b.Config.CmdPrefix):]
						var arg string
						if len(parts) == 1 {
							arg = ""
						} else {
							arg = parts[1]
						}
						log.Printf("Received command %q with arg %q", cmd, arg)
						for _, plugin := range b.Config.Plugins {
							if plugin.Handles(cmd) {
								log.Printf("Plugin %q handling command %q with arg %q", plugin.Name(), cmd, arg)
								if err := plugin.HandleCmd(client, iev, strings.TrimSpace(arg)); err != nil {
									b.Log.Printf("Error: plugin %s: %v", plugin.Name(), err)
								}
							}
						}
					default:
						fmt.Printf("Unhandled inner event: %T %+v\n", iev, iev)
					}
				default:
					fmt.Printf("Unsupported event: %v\n", eventsAPIEvent.Type)
					client.Debugf("unsupported Events API event received")
				}
			default:
				fmt.Printf("Event: %T %+v\n", ev, ev)
			}
		}
	}()
	return client.Run()
}
