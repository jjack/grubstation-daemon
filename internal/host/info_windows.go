//go:build windows

package host

import (
	"net"

	"golang.org/x/sys/windows"
)

func isPhysicalInterface(inf net.Interface) bool {
	var row windows.MibIfRow2
	row.InterfaceIndex = uint32(inf.Index)

	// GetIfEntry2Ex is used as a replacement for GetIfEntry2.
	// Level 0 corresponds to MibIfEntryNormal.
	if err := windows.GetIfEntry2Ex(0, &row); err != nil {
		return true // Fallback to true if we can't determine
	}

	// ConnectorPresent is the first bit of InterfaceAndOperStatusFlags (bit 0)
	// It is TRUE (1) if the interface has a physical connector.
	return (row.InterfaceAndOperStatusFlags & 0x01) != 0
}
