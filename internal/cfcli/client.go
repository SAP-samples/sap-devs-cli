package cfcli

import (
	"encoding/json"
	"os"
	"time"
)

type Runner func(command string) (string, error)

type Client struct {
	run        Runner
	configPath string
	timeout    time.Duration
}

func NewClient(run Runner, configPath string) *Client {
	return &Client{
		run:        run,
		configPath: configPath,
		timeout:    10 * time.Second,
	}
}

type cfConfig struct {
	Target             string `json:"Target"`
	OrganizationFields struct {
		Name string `json:"Name"`
	} `json:"OrganizationFields"`
	SpaceFields struct {
		Name string `json:"Name"`
	} `json:"SpaceFields"`
	AccessToken string `json:"AccessToken"`
}

func (c *Client) readConfig() *cfConfig {
	if c.configPath == "" {
		return nil
	}
	data, err := os.ReadFile(c.configPath)
	if err != nil {
		return nil
	}
	var cfg cfConfig
	if json.Unmarshal(data, &cfg) != nil {
		return nil
	}
	return &cfg
}
