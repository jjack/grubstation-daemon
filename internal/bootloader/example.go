package bootloader

const exampleBootloader = "example"

type Example struct{}

func NewExample() Bootloader {
	return &Example{}
}

func (s *Example) IsActive() bool {
	// you should implement your own logic here to determine if this bootloader is active
	return true
}

func (s *Example) GetBootOptions(configPath string) ([]string, error) {
	return []string{"Ubuntu", "Windows"}, nil
}

func (s *Example) Name() string {
	return exampleBootloader
}
