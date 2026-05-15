//go:build !linux && !windows

package host

import (
	"net"
	"runtime"
)

func isPhysicalInterface(inf net.Interface) bool {
	return true
}

func Platform() string {
	return runtime.GOOS
}
