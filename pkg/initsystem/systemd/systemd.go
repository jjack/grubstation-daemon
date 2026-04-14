package systemd

import (
	"log"

	"github.com/jjack/remote-boot-agent/pkg/initsystem"
)

type SystemdPlugin struct {
	// configuration fields
}

func init() {
	initsystem.Register("systemd", &SystemdPlugin{})
}

func (p *SystemdPlugin) Name() string {
	return "systemd"
}

func (p *SystemdPlugin) RunningServices() ([]string, error) {
	log.Println("Querying systemd for services...")
	// TODO: implement logic
	return []string{}, nil
}
