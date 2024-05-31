package plugins

// PluginConfig is an interface that every plugin has to implement for their
// configuration.
type PluginConfig interface {
	Get(string) string
}
