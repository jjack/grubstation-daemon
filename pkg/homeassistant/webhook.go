package homeassistant

// Message payload for sending to Home Assistant
type WebhookPayload struct {
	DeviceID      string                 `json:"device_id"`
	Bootloader    string                 `json:"bootloader"`
	InitSystem    string                 `json:"init_system"`
	BootOptions   map[string]interface{} `json:"boot_options"`
	// Additional payload parts
}

// Config for connecting to the webhook
type WebhookConfig struct {
	URL   string
	Token string // Or whatever auth is needed
}

// Client object to dispatch to HA
type Client struct {
	config WebhookConfig
}

// NewClient returns a new HA webhook client
func NewClient(config WebhookConfig) *Client {
	return &Client{config: config}
}

// Send dispatches parsed data to the Home Assistant Webhook
func (c *Client) Send(payload WebhookPayload) error {
	// TODO: implement http.Post with json payload
	return nil
}
