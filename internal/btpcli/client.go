package btpcli

import (
	"context"
	"encoding/json"
	"fmt"
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

type btpConfig struct {
	TargetHierarchy struct {
		GlobalAccountSubdomain string `json:"GlobalAccountSubdomain"`
		SubaccountSubdomain    string `json:"SubaccountSubdomain"`
	} `json:"TargetHierarchy"`
	CLIServerURL string `json:"CLIServerURL"`
}

func (c *Client) readConfig() *btpConfig {
	if c.configPath == "" {
		return nil
	}
	data, err := os.ReadFile(c.configPath)
	if err != nil {
		return nil
	}
	var cfg btpConfig
	if json.Unmarshal(data, &cfg) != nil {
		return nil
	}
	return &cfg
}

func (c *Client) runWithContext(ctx context.Context, command string) (string, error) {
	type result struct {
		out string
		err error
	}
	ch := make(chan result, 1)
	go func() {
		out, err := c.run(command)
		ch <- result{out, err}
	}()
	select {
	case <-ctx.Done():
		return "", fmt.Errorf("command timed out after %s — the BTP API may be slow, try again", c.timeout)
	case r := <-ch:
		return r.out, r.err
	}
}
