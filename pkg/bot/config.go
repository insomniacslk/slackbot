package bot

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"

	"github.com/insomniacslk/slack-bots/pkg/credentials"
	"github.com/insomniacslk/slack-bots/plugins"
)

// DefaultCmdPrefix is used when no command prefix is specified in the config
// file.
var DefaultCmdPrefix = "!"

// Config is a configuration object for the bot.
type Config struct {
	BotName     string `json:"bot_name"`
	LogFile     string `json:"logfile"`
	Debug       bool   `json:"debug"`
	Credentials struct {
		PagerDutyAPIKey    string `json:"pagerduty_api_key"`
		SlackAPIKey        string `json:"slack_api_key"`
		SlackAppLevelToken string `json:"slack_app_level_token"`
	} `json:"credentials"`
	CmdPrefix     string                 `json:"cmdprefix,omitempty"`
	PluginConfigs map[string]interface{} `json:"plugins"`

	Plugins []plugins.Plugin `json:"-"`
}

// ReadConfig reads a configuration file and returns a Config object, or an
// error if any.
func ReadConfig(configFile string) (*Config, error) {
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, err
	}
	config := Config{}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// if no command prefix is specified, use the default
	if config.CmdPrefix == "" {
		config.CmdPrefix = DefaultCmdPrefix
	}

	// propagate the API keys to the credentials package, so other plugins can
	// use them.
	credentials.SlackAPIKey = config.Credentials.SlackAPIKey
	credentials.SlackAppLevelToken = config.Credentials.SlackAppLevelToken
	credentials.PagerDutyAPIKey = config.Credentials.PagerDutyAPIKey

	// now parse each plugin
	for name, pconf := range config.PluginConfigs {
		plugin := plugins.Get(name)
		if plugin == nil {
			return nil, fmt.Errorf("unknown plugin %s (did you register it first?)", name)
		}
		// FIXME I don't like this unmarshal/marshal game
		pconfBytes, err := json.Marshal(pconf)
		if err != nil {
			return nil, fmt.Errorf("error marshalling config for plugin %s: %v", name, err)
		}
		if err := plugin.Load(pconfBytes); err != nil {
			return nil, err
		}
		config.Plugins = append(config.Plugins, plugin)
		log.Printf("Loaded plugin %s: %+v", name, pconf)
	}
	return &config, nil
}
