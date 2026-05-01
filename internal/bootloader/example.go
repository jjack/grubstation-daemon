package bootloader

import "context"

const exampleBootloader = "example"

type Example struct{}

func NewExample() Bootloader {
	return &Example{}
}

func (s *Example) IsActive(ctx context.Context) bool {
	// you should implement your own logic here to determine if this bootloader is active
	return true
}

func (s *Example) GetBootOptions(ctx context.Context, cfg Config) ([]string, error) {
	return []string{"Ubuntu", "Windows"}, nil
}

func (s *Example) Name() string {
	return exampleBootloader
}

func (s *Example) Install(ctx context.Context, macAddress, haURL, webhookID string) error {
	return nil
}

func (s *Example) DiscoverConfigPath(ctx context.Context) (string, error) {
	return "/path/to/example.cfg", nil
}
