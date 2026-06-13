package release

import "testing"

func TestParseLatestTag(t *testing.T) {
	tag, err := ParseLatestTag([]byte(`{"tag_name": "v0.2.0", "name": "v0.2.0", "assets": []}`))
	if err != nil || tag != "v0.2.0" {
		t.Errorf("want v0.2.0, got %q (err %v)", tag, err)
	}
	if _, err := ParseLatestTag([]byte(`{"message": "Not Found"}`)); err == nil {
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
