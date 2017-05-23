package vicconfig

type Config struct {
	// Portlayer address in format hostname:port or ip:port
	PortlayerAddress string `toml:"portlayer_address,omitempty"`
}

func DefaultConfig() *Config {
	return &Config{
		PortlayerAddress: "127.0.0.1:2377",
	}
}
