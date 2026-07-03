package release

import (
	"strings"
	"testing"
)

func TestParseLatestTag(t *testing.T) {
	tag, err := ParseLatestTag([]byte(`{"tag_name": "v0.2.0", "name": "v0.2.0", "assets": []}`))
	if err != nil || tag != "v0.2.0" {
		t.Errorf("want v0.2.0, got %q (err %v)", tag, err)
	}
	if _, err := ParseLatestTag([]byte(`{"message": "Not Found"}`)); err == nil {
		t.Error("want error for response without tag_name")
	}
}

func TestParseLatest(t *testing.T) {
	rel, err := ParseLatest([]byte(`{"tag_name": "v0.2.0", "name": "v0.2.0 — doctor", "body": "## Changes\n- doctor view", "html_url": "https://github.com/mikevalstar/myplace/releases/tag/v0.2.0", "published_at": "2026-06-16T12:00:00Z"}`))
	if err != nil {
		t.Fatal(err)
	}
	if rel.Tag != "v0.2.0" || rel.Name != "v0.2.0 — doctor" || !strings.Contains(rel.Notes, "doctor view") {
		t.Errorf("unexpected release: %+v", rel)
	}
	if rel.PublishedAt.IsZero() || rel.URL == "" {
		t.Errorf("published_at/html_url should parse: %+v", rel)
	}
	if _, err := ParseLatest([]byte(`{"message": "Not Found"}`)); err == nil {
		t.Error("want error for response without tag_name")
	}
}

func TestNormalizeTag(t *testing.T) {
	if NormalizeTag("v0.2.0") != "0.2.0" || NormalizeTag("0.2.0") != "0.2.0" {
		t.Error("NormalizeTag should strip a single leading v")
	}
}

func TestParseChecksums(t *testing.T) {
	body := []byte("aaa111  myplace_linux_amd64.tar.gz\n" +
		"bbb222  myplace_darwin_arm64.tar.gz\n")
	sum, err := ParseChecksums(body, "myplace_darwin_arm64.tar.gz")
	if err != nil || sum != "bbb222" {
		t.Errorf("want bbb222, got %q (err %v)", sum, err)
	}
	if _, err := ParseChecksums(body, "myplace_windows_amd64.tar.gz"); err == nil {
		t.Error("want error when the filename is absent")
	}
}

func TestEqualHex(t *testing.T) {
	if !equalHex("ABCD", "abcd") {
		t.Error("equalHex should ignore case")
	}
	if equalHex("", "") || equalHex("ab", "cd") || equalHex("zz", "zz") {
		t.Error("equalHex should reject empty, differing, and non-hex inputs")
	}
}
