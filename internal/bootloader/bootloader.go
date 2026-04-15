package bootloader

// BootOptions represents the parsed boot configuration.
type BootOptions struct {
	AvailableOSes []string
	Parameters    map[string]string
	// Add other relevant fields
}

// Plugin defines the interface for all bootloader plugins.
type Plugin interface {
	// Name returns the name of the bootloader plugin.
	Name() string
	// Detect returns true if this bootloader is detected as the active/available one on the system.
	Detect() bool
	// Parse parses the bootloader configuration and returns the options.
	Parse(configPath string) (*BootOptions, error)
	// Add other necessary methods (e.g., SetNextBoot)
}

// Registry manages the available bootloader plugins
type Registry struct {
	plugins map[string]Plugin
}

// NewRegistry creates a new bootloader registry with the provided plugins
func NewRegistry(plugins ...Plugin) *Registry {
	r := &Registry{plugins: make(map[string]Plugin)}
	for _, p := range plugins {
		r.plugins[p.Name()] = p
	}
	return r
}

// Get returns a registered bootloader plugin by name.
func (r *Registry) Get(name string) (Plugin, bool) {
	p, ok := r.plugins[name]
	return p, ok
}

// Detect iterates over all registered plugins and returns the name of the first one that detects itself.
func (r *Registry) Detect() string {
	for name, plugin := range r.plugins {
		if plugin.Detect() {
			return name
		}
	}
	return ""
}
