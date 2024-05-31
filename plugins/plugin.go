package plugins

import (
	"fmt"
	"log"
	"sync"

	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

// Plugin is the interface that every plugin must implement.
type Plugin interface {
	Name() string
	HandleCmd(*socketmode.Client, *slackevents.MessageEvent, string) error
	Load([]byte) error
	Handles(string) bool
}

type _plugins struct {
	registered map[string]Plugin
	mutex      sync.Mutex
}

var plugins *_plugins

func init() {
	plugins = &_plugins{
		registered: make(map[string]Plugin),
	}
}

func (p *_plugins) register(name string, plugin Plugin) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if _, present := p.registered[name]; present {
		return fmt.Errorf("plugin %s already registered", name)
	}
	p.registered[name] = plugin
	log.Printf("registered plugin %s", name)
	return nil
}

func (p *_plugins) get(name string) Plugin {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.registered[name]
}

// Get returns a plugin object, if registered, or nil.
func Get(name string) Plugin {
	return plugins.get(name)
}

// Register adds a plugin by name to the map of registered plugins. If a plugin
// with the same name is registered already, an error is returned.
func Register(name string, plugin Plugin) error {
	return plugins.register(name, plugin)
}
