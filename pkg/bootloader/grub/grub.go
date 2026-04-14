package grub

import (
	"log"

	"github.com/jjack/remote-boot-agent/pkg/bootloader"
)

type GrubPlugin struct {
	// Add config for the grub plugin here
}

func init() {
	bootloader.Register("grub", &GrubPlugin{})
}

func (p *GrubPlugin) Name() string {
	return "grub"
}

func (p *GrubPlugin) Parse() (*bootloader.BootOptions, error) {
	log.Println("Parsing GRUB boot options...")
	// TODO: implement GRUB parsing logic
	return &bootloader.BootOptions{}, nil
}
