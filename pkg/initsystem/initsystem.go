package initsystem

// InitSystem defines the interface for init system plugins.
type InitSystem interface {
	Name() string
	RunningServices() ([]string, error)
	// Add more methods interacting with init systems...
}

// Registry to keep track of init system plugins
var plugins = make(map[string]InitSystem)

// Register makes an init system plugin available by the provided name.
func Register(name string, plugin InitSystem) {
	if plugin == nil {
		panic("initsystem: Register plugin is nil")
	}
	if _, dup := plugins[name]; dup {
		panic("initsystem: Register called twice for plugin " + name)
	}
	plugins[name] = plugin
}

// Get returns a registered init system plugin by name.
func Get(name string) (InitSystem, bool) {
	p, ok := plugins[name]
	return p, ok
}
