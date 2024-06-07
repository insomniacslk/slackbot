package bot

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/insomniacslk/slackbot/pkg/credentials"
	"github.com/insomniacslk/slackbot/plugins"
	"github.com/mitchellh/go-homedir"
)

// DefaultCmdPrefix is used when no command prefix is specified in the config
// file.
var DefaultCmdPrefix = "!"

// Config is a configuration object for the bot.
type Config struct {
	BotName     string `mapstructure:"bot_name"`
	LogFile     string `mapstructure:"logfile"`
	Debug       bool   `mapstructure:"debug"`
	Credentials struct {
		PagerDutyAPIKey    string `mapstructure:"pagerduty_api_key"`
		SlackBotToken      string `mapstructure:"slack_bot_token"`
		SlackAppLevelToken string `mapstructure:"slack_app_level_token"`
	} `mapstructure:"credentials"`
	CmdPrefix     string                 `mapstructure:"cmdprefix,omitempty"`
	PluginConfigs map[string]interface{} `mapstructure:"plugins"`

	Plugins []plugins.Plugin `mapstructure:"-"`
}

func (c *Config) Validate() error {
	// expand logfile
	lf, err := homedir.Expand(c.LogFile)
	if err != nil {
		return fmt.Errorf("failed to expand log_file path: %w", err)
	}
	c.LogFile = lf

	// if no command prefix is specified, use the default
	if c.CmdPrefix == "" {
		c.CmdPrefix = DefaultCmdPrefix
	}
	// propagate the API keys to the credentials package, so other plugins can
	// use them.
	credentials.SlackBotToken = c.Credentials.SlackBotToken
	credentials.SlackAppLevelToken = c.Credentials.SlackAppLevelToken
	credentials.PagerDutyAPIKey = c.Credentials.PagerDutyAPIKey

	// now parse each plugin
	for name, pconf := range c.PluginConfigs {
		plugin := plugins.Get(name)
		if plugin == nil {
			return fmt.Errorf("unknown plugin %s (did you register it first?)", name)
		}
		// FIXME I don't like this unmarshal/marshal game
		pconfBytes, err := json.Marshal(pconf)
		if err != nil {
			return fmt.Errorf("error marshalling config for plugin %s: %v", name, err)
		}
		if err := plugin.Load(pconfBytes); err != nil {
			return fmt.Errorf("failed to load plugin: %w", err)
		}
		c.Plugins = append(c.Plugins, plugin)
		log.Printf("Loaded plugin %s: %+v", name, pconf)
	}
	return nil
}
