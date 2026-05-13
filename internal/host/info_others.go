//go:build !linux && !windows

package host

import (
	"net"
)

func isPhysicalInterface(inf net.Interface) bool {
	return true
}
