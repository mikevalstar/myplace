// Package release talks to GitHub Releases: latest-version lookup for status
// and the self-update binary swap. Archive naming and URLs are pinned by
// ADR-0004 — change them there first.
package release

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
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

// ArchiveName is the version-less archive filename for this platform — also the
// name used in the release's checksums.txt (ADR-0004).
func ArchiveName() string {
	return fmt.Sprintf("myplace_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
}

// ArchiveURL is the permanent download URL for the newest release on this
// platform (version-less naming per ADR-0004 — no API call needed).
func ArchiveURL() string {
	return fmt.Sprintf("https://github.com/%s/releases/latest/download/%s", Repo, ArchiveName())
}

// ChecksumsURL is the permanent download URL for the newest release's
// checksums.txt (goreleaser default name, ADR-0004).
func ChecksumsURL() string {
	return fmt.Sprintf("https://github.com/%s/releases/latest/download/checksums.txt", Repo)
}

// ParseChecksums extracts the hex sha256 for filename from a goreleaser
// checksums.txt body (each line is "<hex>  <filename>", sha256sum format).
func ParseChecksums(body []byte, filename string) (string, error) {
	for _, line := range strings.Split(string(body), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == filename {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("no checksum for %s in checksums.txt", filename)
}

// download fetches url fully into memory, capped at limit bytes.
func download(ctx context.Context, url string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("downloading %s: HTTP %d", url, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, limit))
}

// equalHex compares two hex checksums by decoded bytes, so casing or stray
// whitespace can't cause a false mismatch.
func equalHex(a, b string) bool {
	ab, err1 := hex.DecodeString(strings.TrimSpace(a))
	bb, err2 := hex.DecodeString(strings.TrimSpace(b))
	return err1 == nil && err2 == nil && len(ab) > 0 && bytes.Equal(ab, bb)
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

	// Download the whole archive into memory so its checksum can be verified
	// before anything touches the running binary. Releases are a single small
	// binary; the 64 MiB cap is generous headroom against a runaway body.
	archive, err := download(ctx, ArchiveURL(), 64<<20)
	if err != nil {
		return "", err
	}

	// Verify against the release's checksums.txt before extracting — this code
	// overwrites its own executable, so TLS trust alone isn't enough (ADR-0004).
	sums, err := download(ctx, ChecksumsURL(), 1<<20)
	if err != nil {
		return "", fmt.Errorf("fetching checksums: %w", err)
	}
	want, err := ParseChecksums(sums, ArchiveName())
	if err != nil {
		return "", err
	}
	got := fmt.Sprintf("%x", sha256.Sum256(archive))
	if !equalHex(got, want) {
		return "", fmt.Errorf("checksum mismatch for %s: got %s, want %s", ArchiveName(), got, want)
	}

	gz, err := gzip.NewReader(bytes.NewReader(archive))
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
