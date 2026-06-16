// Package chezmoi wraps the chezmoi CLI. It never reimplements chezmoi
// behavior — it invokes the binary and parses output.
package chezmoi

import (
	"context"
	"encoding/json"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/mikevalstar/myplace/internal/run"
)

// dottedVersion matches the first "X.Y.Z" in a tool's --version banner.
var dottedVersion = regexp.MustCompile(`\d+\.\d+\.\d+`)

type Client struct {
	r run.Runner
}

func New(r run.Runner) *Client { return &Client{r: r} }

type MachineData struct {
	Profile string `json:"profile"`
	Push    *bool  `json:"push,omitempty"`
}

type PushPolicy struct {
	Allowed bool
	Source  string
}

func PushPolicyForData(data MachineData) PushPolicy {
	if data.Push != nil {
		return PushPolicy{Allowed: *data.Push, Source: "data.push"}
	}
	if data.Profile == "server" {
		return PushPolicy{Allowed: false, Source: "profile:server"}
	}
	return PushPolicy{Allowed: true, Source: "default"}
}

// cz runs chezmoi with --no-tty always set, so a conflict prompt can never
// open /dev/tty and hang a caller (notably the TUI, which owns the terminal).
// With no TTY and a closed stdin, chezmoi fails fast instead of blocking.
func (c *Client) cz(ctx context.Context, args ...string) ([]byte, error) {
	return c.r.Run(ctx, "", "chezmoi", append([]string{"--no-tty"}, args...)...)
}

// Installed reports whether the chezmoi binary is available.
func (c *Client) Installed(ctx context.Context) bool {
	_, err := c.cz(ctx, "--version")
	return err == nil
}

// Initialized reports whether this machine has a chezmoi source directory
// with actual content. source-path prints the default path even before init,
// and some chezmoi commands auto-create the directory empty — so require a
// non-empty dir.
func (c *Client) Initialized(ctx context.Context) bool {
	out, err := c.cz(ctx, "source-path")
	if err != nil {
		return false
	}
	p := strings.TrimSpace(string(out))
	if p == "" {
		return false
	}
	entries, err := os.ReadDir(p)
	return err == nil && len(entries) > 0
}

// Version returns chezmoi's version as a dotted number ("2.70.5"), parsed from
// `chezmoi --version` (banner: "chezmoi version v2.70.5, commit ..."). Empty
// string if the banner has no recognizable version.
func (c *Client) Version(ctx context.Context) (string, error) {
	out, err := c.cz(ctx, "--version")
	if err != nil {
		return "", err
	}
	return dottedVersion.FindString(string(out)), nil
}

// RemoteURL is the origin URL of the dotfiles source repo (empty if unset).
func (c *Client) RemoteURL(ctx context.Context) (string, error) {
	out, err := c.cz(ctx, "git", "--", "remote", "get-url", "origin")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// LsRemote contacts the source repo's origin (network) without changing
// anything — a read-only reachability probe for `doctor`.
func (c *Client) LsRemote(ctx context.Context) error {
	_, err := c.cz(ctx, "git", "--", "ls-remote", "--exit-code", "origin", "HEAD")
	return err
}

// Data returns the myplace machine data from chezmoi's template data
// (set by home/.chezmoi.toml.tmpl on init).
func (c *Client) Data(ctx context.Context) (MachineData, error) {
	out, err := c.cz(ctx, "data", "--format", "json")
	if err != nil {
		return MachineData{}, err
	}
	var data MachineData
	if err := json.Unmarshal(out, &data); err != nil {
		return MachineData{}, err
	}
	return data, nil
}

// Profile returns the machine profile from chezmoi's template data.
func (c *Client) Profile(ctx context.Context) (string, error) {
	data, err := c.Data(ctx)
	if err != nil {
		return "", err
	}
	return data.Profile, nil
}

// PushPolicy reports whether this profile is allowed to push captured source
// repo commits. An explicit data.push wins; older bootstraps without it use
// the current profile default: servers consume, Macs may originate changes.
func (c *Client) PushPolicy(ctx context.Context) (PushPolicy, error) {
	data, err := c.Data(ctx)
	if err != nil {
		return PushPolicy{}, err
	}
	return PushPolicyForData(data), nil
}

// FileStatus is one line of `chezmoi status`.
type FileStatus struct {
	Path string
	// LocalChanged: the file on disk changed since chezmoi last wrote it
	// (outgoing drift — first status column).
	LocalChanged bool
	// ApplyChanges: `chezmoi apply` would modify the file
	// (incoming drift — second status column).
	ApplyChanges bool
}

// ParseStatus parses `chezmoi status` output: two status columns then the path.
func ParseStatus(out []byte) []FileStatus {
	var files []FileStatus
	for _, line := range strings.Split(string(out), "\n") {
		if len(line) < 4 {
			continue
		}
		files = append(files, FileStatus{
			Path:         strings.TrimSpace(line[3:]),
			LocalChanged: line[0] != ' ',
			ApplyChanges: line[1] != ' ',
		})
	}
	return files
}

func (c *Client) Status(ctx context.Context) ([]FileStatus, error) {
	out, err := c.cz(ctx, "status")
	if err != nil {
		return nil, err
	}
	return ParseStatus(out), nil
}

// Fetch updates remote tracking refs in the source repo (network).
func (c *Client) Fetch(ctx context.Context) error {
	_, err := c.cz(ctx, "git", "--", "fetch", "--quiet")
	return err
}

func (c *Client) revListCount(ctx context.Context, rang string) (int, error) {
	out, err := c.cz(ctx, "git", "--", "rev-list", "--count", rang)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(out)))
}

// BehindUpstream is the number of commits on origin not yet in the local source repo.
func (c *Client) BehindUpstream(ctx context.Context) (int, error) {
	return c.revListCount(ctx, "HEAD..@{upstream}")
}

// AheadUpstream is the number of local source-repo commits not yet pushed.
func (c *Client) AheadUpstream(ctx context.Context) (int, error) {
	return c.revListCount(ctx, "@{upstream}..HEAD")
}

// Uncommitted is the number of dirty paths in the source repo working tree.
func (c *Client) Uncommitted(ctx context.Context) (int, error) {
	out, err := c.cz(ctx, "git", "--", "status", "--porcelain")
	if err != nil {
		return 0, err
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return 0, nil
	}
	return len(strings.Split(s, "\n")), nil
}

// Update pulls the source repo and applies the result (chezmoi update).
func (c *Client) Update(ctx context.Context) error {
	_, err := c.cz(ctx, "update")
	return err
}

// Pull updates the source repo without applying it to the target files.
func (c *Client) Pull(ctx context.Context) error {
	_, err := c.cz(ctx, "git", "--", "pull", "--rebase")
	return err
}

// Apply applies the current source state to all managed target files.
func (c *Client) Apply(ctx context.Context) error {
	_, err := c.cz(ctx, "apply")
	return err
}

// Diff shows what `chezmoi apply` would change for one target (absolute path).
// For a locally-modified file this is the change that would UNDO the local edit.
func (c *Client) Diff(ctx context.Context, target string) (string, error) {
	out, err := c.cz(ctx, "diff", target)
	return string(out), err
}

// ApplyTarget applies the current source state to one managed target file.
func (c *Client) ApplyTarget(ctx context.Context, target string) error {
	_, err := c.cz(ctx, "apply", target)
	return err
}

// ReAdd captures the local state of a target back into the source repo
// (machine wins).
func (c *Client) ReAdd(ctx context.Context, target string) error {
	_, err := c.cz(ctx, "re-add", target)
	return err
}

// ApplyForce overwrites a target with the source state (repo wins).
func (c *Client) ApplyForce(ctx context.Context, target string) error {
	_, err := c.cz(ctx, "apply", "--force", target)
	return err
}

// CommitAll stages and commits everything in the source repo.
func (c *Client) CommitAll(ctx context.Context, message string) error {
	if _, err := c.cz(ctx, "git", "--", "add", "-A"); err != nil {
		return err
	}
	_, err := c.cz(ctx, "git", "--", "commit", "-m", message)
	return err
}

// Push pushes the source repo to its upstream.
func (c *Client) Push(ctx context.Context) error {
	_, err := c.cz(ctx, "git", "--", "push")
	return err
}

// InitApply runs first-time setup: clone the repo, answer the init prompts
// non-interactively, and apply. profile is the machine profile; promptStrings
// pre-answers any promptStringOnce values (e.g. gitEmail) so init never blocks.
func (c *Client) InitApply(ctx context.Context, repo, profile string, promptStrings map[string]string) error {
	args := []string{"init", "--apply"}
	if profile != "" {
		args = append(args, "--promptChoice", "profile="+profile)
	}
	// Sorted for a stable command line (logs/tests).
	for _, k := range sortedKeys(promptStrings) {
		if promptStrings[k] != "" {
			args = append(args, "--promptString", k+"="+promptStrings[k])
		}
	}
	args = append(args, repo)
	_, err := c.cz(ctx, args...)
	return err
}

func sortedKeys(m map[string]string) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}
