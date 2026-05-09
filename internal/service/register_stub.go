//go:build !linux && !windows

package service

// RegisterDefaultServices is a no-op fallback for unsupported platforms.
func RegisterDefaultServices(r *Registry) {
	// No supported services for this platform
}
