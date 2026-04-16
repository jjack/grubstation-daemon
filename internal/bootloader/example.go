package bootloader

const exampleBootloader = "example"

func init() {
	Register(exampleBootloader, NewExample)
}

type Example struct{}

func NewExample() Bootloader {
	return &Example{}
}

func (s *Example) IsActive() bool {
	return true
}

func (s *Example) GetBootOptions(configPath string) ([]string, error) {
	return []string{"Ubuntu", "Windows"}, nil
}

func (s *Example) Name() string {
	return exampleBootloader
}
