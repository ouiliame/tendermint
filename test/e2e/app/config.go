package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Listen          string
	GRPC            bool `toml:"grpc"`
	File            string
	PersistInterval uint64 `toml:"persist_interval"`
}

func LoadConfig(file string) (*Config, error) {
	cfg := &Config{
		Listen:          "unix:///var/run/app.sock",
		GRPC:            false,
		PersistInterval: 1,
	}
	r, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("failed to open app config %q: %w", file, err)
	}
	_, err = toml.DecodeReader(r, &cfg)
	if err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return cfg, cfg.Validate()
}

func (cfg Config) Validate() error {
	if cfg.Listen == "" {
		return errors.New("listen parameter is required")
	}
	return nil
}
