package actions

import (
	"fmt"
	"os"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
)

// Say makes the bot speak on Slack.
func Say(client *socketmode.Client, dest string, fmts string, args ...interface{}) {
	if _, _, err := client.PostMessage(dest, slack.MsgOptionText(fmt.Sprintf(fmts, args...), false)); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to post message: %v\n", err)
	}
}
