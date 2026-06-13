// Package release talks to GitHub Releases: latest-version lookup for status
// and the self-update binary swap. Archive naming and URLs are pinned by
// ADR-0004 — change them there first.
package release

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const Repo = "mikevalstar/myplace"

// NormalizeTag strips the leading "v" so tags compare against the
// ldflags-stamped version.
func NormalizeTag(tag string) string { return strings.TrimPrefix(tag, "v") }

// ParseLatestTag extracts tag_name from a GitHub releases/latest response.
func ParseLatestTag(body []byte) (string, error) {
	var resp struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", err
	}
	if resp.TagName == "" {
		return "", errors.New("no tag_name in release response")
	}
	return resp.TagName, nil
}

// LatestTag queries the GitHub API for the newest release tag (e.g. "v0.1.0").
func LatestTag(ctx context.Context) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", Repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("releases/latest: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	return ParseLatestTag(body)
}

// ArchiveURL is the permanent download URL for the newest release on this
// platform (version-less naming per ADR-0004 — no API call needed).
func ArchiveURL() string {
	return fmt.Sprintf("https://github.com/%s/releases/latest/download/myplace_%s_%s.tar.gz",
		Repo, runtime.GOOS, runtime.GOARCH)
}

// SelfUpdate replaces the running binary with the latest release.
// Returns the tag it updated to; ErrUpToDate when already current.
var ErrUpToDate = errors.New("already up to date")

func SelfUpdate(ctx context.Context, currentVersion string) (string, error) {
	tag, err := LatestTag(ctx)
	if err != nil {
		return "", fmt.Errorf("checking latest release: %w", err)
	}
	if NormalizeTag(tag) == currentVersion {
		return tag, ErrUpToDate
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ArchiveURL(), nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("downloading %s: HTTP %d", ArchiveURL(), resp.StatusCode)
	}

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return "", err
	}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return "", errors.New("myplace binary not found in release archive")
		}
		if err != nil {
			return "", err
		}
		if filepath.Base(hdr.Name) == "myplace" && hdr.Typeflag == tar.TypeReg {
			break
		}
	}

	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	// Write beside the target so the final rename is atomic (same filesystem).
	tmp, err := os.CreateTemp(filepath.Dir(exe), ".myplace-update-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, tr); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Chmod(0o755); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmp.Name(), exe); err != nil {
		return "", err
	}
	return tag, nil
}
