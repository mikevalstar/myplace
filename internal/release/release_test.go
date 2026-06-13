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
