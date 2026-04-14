package bootloader

// BootOptions represents the parsed boot configuration.
type BootOptions struct {
	KernelPaths []string
	Parameters  map[string]string
	// Add other relevant fields
}

// Bootloader defines the interface for all bootloader plugins.
type Bootloader interface {
	// Name returns the name of the bootloader plugin.
	Name() string
	// Parse parses the bootloader configuration and returns the options.
	Parse() (*BootOptions, error)
	// Add other necessary methods (e.g., SetNextBoot)
}

// Registry to keep track of available plugins
var plugins = make(map[string]Bootloader)

// Register makes a bootloader plugin available by the provided name.
func Register(name string, plugin Bootloader) {
	if plugin == nil {
		panic("bootloader: Register plugin is nil")
	}
	if _, dup := plugins[name]; dup {
		panic("bootloader: Register called twice for plugin " + name)
	}
	plugins[name] = plugin
}

// Get returns a registered bootloader plugin by name.
func Get(name string) (Bootloader, bool) {
	p, ok := plugins[name]
	return p, ok
}
