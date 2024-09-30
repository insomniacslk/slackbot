package actions

import (
	"fmt"
	"os"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
)

// Say makes the bot speak on Slack.
func Say(client *socketmode.Client, dest string, threadTS, fmts string, args ...interface{}) {
	if _, _, err := client.PostMessage(
		dest,
		slack.MsgOptionText(fmt.Sprintf(fmts, args...), false),
		// if threadTS is an empty string, the message is posted on the main channel/thread
		slack.MsgOptionTS(threadTS),
	); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to post message: %v\n", err)
	}
}
