// Package mise wraps the mise CLI. All commands run from the user's home
// directory so only the GLOBAL config (~/.config/mise/config.toml — managed
// by chezmoi) applies, never a project-local mise.toml.
package mise

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/mikevalstar/myplace/internal/run"
)

type Client struct {
	r    run.Runner
	home string
}

func New(r run.Runner) *Client {
	home, _ := os.UserHomeDir()
	return &Client{r: r, home: home}
}

func (c *Client) Installed(ctx context.Context) bool {
	_, err := c.r.Run(ctx, c.home, "mise", "version")
	return err == nil
}

// Tool is one entry from `mise ls --json`.
type Tool struct {
	Name      string
	Version   string
	Requested string
	Installed bool
}

// ParseLs parses `mise ls --json`: a map of tool name to version entries.
// Unknown fields are ignored; a missing "installed" key counts as installed.
func ParseLs(out []byte) ([]Tool, error) {
	var raw map[string][]map[string]any
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, err
	}
	var tools []Tool
	for name, entries := range raw {
		for _, e := range entries {
			t := Tool{Name: name, Installed: true}
			if v, ok := e["version"].(string); ok {
				t.Version = v
			}
			if v, ok := e["requested_version"].(string); ok {
				t.Requested = v
			}
			if v, ok := e["installed"].(bool); ok {
				t.Installed = v
			}
			tools = append(tools, t)
		}
	}
	return tools, nil
}

func (c *Client) Ls(ctx context.Context) ([]Tool, error) {
	out, err := c.r.Run(ctx, c.home, "mise", "ls", "--json")
	if err != nil {
		return nil, err
	}
	return ParseLs(out)
}

// Outdated is one entry from `mise outdated --json`.
type Outdated struct {
	Name    string `json:"name"`
	Current string `json:"current"`
	Wanted  string `json:"wanted"`
}

// ParseOutdated parses `mise outdated --json`: a map of tool name to
// {current, requested, latest, ...}. Tools with no current version are
// skipped — they're reported as missing by Ls, not outdated.
func ParseOutdated(out []byte) ([]Outdated, error) {
	var raw map[string]map[string]any
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, err
	}
	var result []Outdated
	for name, e := range raw {
		o := Outdated{Name: name}
		if v, ok := e["current"].(string); ok {
			o.Current = v
		}
		if v, ok := e["latest"].(string); ok {
			o.Wanted = v
		} else if v, ok := e["bump"].(string); ok {
			o.Wanted = v
		}
		if o.Current == "" {
			continue
		}
		result = append(result, o)
	}
	return result, nil
}

func (c *Client) Outdated(ctx context.Context) ([]Outdated, error) {
	out, err := c.r.Run(ctx, c.home, "mise", "outdated", "--json")
	if err != nil && len(out) == 0 {
		return nil, err
	}
	return ParseOutdated(out)
}

// Trust marks the global config as trusted (best-effort; required before
// install on a fresh machine).
func (c *Client) Trust(ctx context.Context) {
	cfg := filepath.Join(c.home, ".config", "mise", "config.toml")
	_, _ = c.r.Run(ctx, c.home, "mise", "trust", cfg)
}

// Install installs anything in the global config that's missing.
func (c *Client) Install(ctx context.Context) error {
	_, err := c.r.Run(ctx, c.home, "mise", "install")
	return err
}

// Upgrade brings installed tools up to the versions the config requests.
// It does not bump the config itself — converge, don't mutate.
func (c *Client) Upgrade(ctx context.Context) error {
	_, err := c.r.Run(ctx, c.home, "mise", "upgrade")
	return err
}
