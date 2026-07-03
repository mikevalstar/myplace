// Package shelly wraps the Shelly CLI (Arch/CachyOS's all-in-one package
// helper) for read-only inventory. Like internal/brew it is present-if-installed:
// callers check Installed first and skip Shelly entirely when it is not on PATH,
// so this package is safe to wire up on every OS (it self-reports unavailable on
// macOS/servers). It never installs, upgrades, syncs, or otherwise mutates
// anything — it only reads the available-updates list. `shelly check-updates`
// covers native Arch repos by default; the AUR and Flatpak channels are opt-in
// per invocation (`-a`/`-l`), which we pass so a single call reports all three.
// (AppImage has no check-updates flag, so it is out of scope here.) The adapter
// that turns this into an outdated.Source lives in internal/outdated, so this
// package stays a thin CLI wrapper with no cross-imports (ADR-0010).
package shelly

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/mikevalstar/myplace/internal/run"
)

type Client struct {
	r run.Runner
}

func New(r run.Runner) *Client {
	return &Client{r: r}
}

// Installed reports whether shelly is on PATH. Cheap and side-effect-free.
func (c *Client) Installed(ctx context.Context) bool {
	_, err := c.r.Run(ctx, "", "shelly", "--version")
	return err == nil
}

// Package is one outdated package. Field names match outdated.Package so the
// adapter in internal/outdated is a trivial conversion.
type Package struct {
	Name    string
	Current string
	Latest  string
}

// shellyDoc is the confirmed shape of `shelly check-updates -a -l --json`
// (captured from Shelly 2.4.1 on CachyOS). It is an object with a MetaData
// header and one array per channel; the repo/AUR entries carry Name + Version
// (the available/new version) + OldVersion (currently installed), while Flatpak
// entries carry Id + Name + Version and have no old-version field.
type shellyDoc struct {
	Packages []shellyEntry `json:"Packages"` // native Arch repos
	Aur      []shellyEntry `json:"Aur"`
	Flatpak  []shellyEntry `json:"Flatpak"`
}

type shellyEntry struct {
	Name       string `json:"Name"`
	ID         string `json:"Id"`         // Flatpak app id; empty for repo/AUR
	Version    string `json:"Version"`    // the available/new version
	OldVersion string `json:"OldVersion"` // installed version; empty for Flatpak
}

// ParseOutdated parses `shelly check-updates -a -l --json` (native repos + AUR +
// Flatpak). Shelly prints a human "Checking for updates..." line before the JSON
// body on stdout, so we skip to the first `{`/`[` rather than assuming the buffer
// starts with JSON.
//
// Packages from a secondary channel (AUR, Flatpak) are prefixed with that channel
// (e.g. `flatpak:org.gimp.GIMP`) so they stay distinguishable once every channel
// is flattened into the single "shelly" source row; native repo packages keep
// their bare name (as pacman shows them). An entry is kept when it has a name and
// an available version; Flatpak carries no installed version, so — unlike the
// brew/mise empty-current skip — a blank Current does not drop it (the row still
// tells you an update is waiting).
func ParseOutdated(out []byte) ([]Package, error) {
	trimmed := bytes.TrimSpace(out)
	if len(trimmed) == 0 {
		return nil, nil
	}
	i := bytes.IndexByte(trimmed, '{')
	if i < 0 {
		// Non-empty output with no JSON object at all (e.g. an error message):
		// don't silently report "nothing outdated".
		return nil, fmt.Errorf("shelly: no JSON object in output: %q", firstLine(trimmed))
	}
	body := trimmed[i:]

	var doc shellyDoc
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}

	var pkgs []Package
	add := func(entries []shellyEntry, prefix string) {
		for _, e := range entries {
			name := e.Name
			if name == "" {
				name = e.ID // Flatpak entries may only carry an id
			}
			if name == "" || e.Version == "" {
				continue
			}
			pkgs = append(pkgs, Package{Name: prefix + name, Current: e.OldVersion, Latest: e.Version})
		}
	}
	add(doc.Packages, "")
	add(doc.Aur, "aur:")
	add(doc.Flatpak, "flatpak:")
	return pkgs, nil
}

// firstLine returns the first line of b, for use in error messages.
func firstLine(b []byte) string {
	if i := bytes.IndexByte(b, '\n'); i >= 0 {
		return string(b[:i])
	}
	return string(b)
}

// Outdated returns every package with a newer version available across the repo,
// AUR, and Flatpak channels. `-a`/`-l` opt the AUR and Flatpak channels into the
// otherwise repo-only check. It reads only — it never runs `shelly sync`/`update`/
// `upgrade`, so freshness reflects the user's last sync (same read-only stance as
// brew.Outdated, which never runs `brew update`). Shelly may exit non-zero while
// still emitting valid JSON, so a non-empty body is trusted even on error (same
// tolerance as brew/mise).
func (c *Client) Outdated(ctx context.Context) ([]Package, error) {
	out, err := c.r.Run(ctx, "", "shelly", "check-updates", "-a", "-l", "--json")
	if err != nil && len(out) == 0 {
		return nil, err
	}
	return ParseOutdated(out)
}
