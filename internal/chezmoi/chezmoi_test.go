package chezmoi

import "testing"

func TestParseStatus(t *testing.T) {
	out := []byte(" M .zshrc\nMM .config/mise/config.toml\n A .config/new\n\n")
	files := ParseStatus(out)
	if len(files) != 3 {
		t.Fatalf("want 3 files, got %d: %v", len(files), files)
	}

	zshrc := files[0]
	if zshrc.Path != ".zshrc" || zshrc.LocalChanged || !zshrc.ApplyChanges {
		t.Errorf("zshrc: want incoming-only change, got %+v", zshrc)
	}

	both := files[1]
	if !both.LocalChanged || !both.ApplyChanges {
		t.Errorf("mise config: want changes in both directions, got %+v", both)
	}

	added := files[2]
	if added.Path != ".config/new" || added.LocalChanged || !added.ApplyChanges {
		t.Errorf("new file: want incoming add, got %+v", added)
	}
}

func TestParseStatusEmpty(t *testing.T) {
	if files := ParseStatus([]byte("")); len(files) != 0 {
		t.Errorf("want no files from empty output, got %v", files)
	}
}
