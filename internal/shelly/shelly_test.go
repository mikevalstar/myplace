package shelly

import "testing"

// Real shape of `shelly check-updates -a -l --json`, captured from Shelly 2.4.1
// on CachyOS: a "Checking for updates..." status line precedes the JSON body,
// which is an object with a MetaData header and one array per channel. Repo/AUR
// entries carry Name + Version (available) + OldVersion (installed); Flatpak
// entries carry Id + Name + Version and no old-version field.
const realOutput = `Checking for updates...
{"MetaData":{"Version":"v1","Date":"07/03/2026","Time":1783115969},"Packages":[{"Name":"fish","Version":"4.8.0-1","OldVersion":"4.7.1-1","DownloadSize":"3.56 MiB"},{"Name":"mesa","Version":"3:26.1.4-1","OldVersion":"3:26.1.3-1","DownloadSize":"13.34 MiB"}],"Aur":[{"Name":"yay","Version":"12.4.1-1","OldVersion":"12.4.0-1"}],"Flatpak":[{"Id":"org.gtk.Gtk3theme.Breeze","Name":"Breeze GTK theme","Version":"6.7.1"}]}`

func TestParseOutdated(t *testing.T) {
	pkgs, err := ParseOutdated([]byte(realOutput))
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 4 {
		t.Fatalf("want 4 packages (2 repo + 1 aur + 1 flatpak), got %d: %v", len(pkgs), pkgs)
	}
	byName := map[string]Package{}
	for _, p := range pkgs {
		byName[p.Name] = p
	}
	// Native repo package: bare name, OldVersion→Current, Version→Latest.
	if f := byName["fish"]; f.Current != "4.7.1-1" || f.Latest != "4.8.0-1" {
		t.Errorf("fish: got %+v, want current 4.7.1-1 / latest 4.8.0-1", f)
	}
	// AUR package: channel-prefixed.
	if y, ok := byName["aur:yay"]; !ok {
		t.Errorf("AUR package should be channel-prefixed as aur:yay, got %v", pkgs)
	} else if y.Current != "12.4.0-1" || y.Latest != "12.4.1-1" {
		t.Errorf("aur:yay: got %+v, want current 12.4.0-1 / latest 12.4.1-1", y)
	}
	// Flatpak: channel-prefixed by Name, no installed version but still kept.
	if g, ok := byName["flatpak:Breeze GTK theme"]; !ok {
		t.Errorf("flatpak package should be channel-prefixed, got %v", pkgs)
	} else if g.Current != "" || g.Latest != "6.7.1" {
		t.Errorf("flatpak: got %+v, want empty current / latest 6.7.1", g)
	}
}

func TestParseOutdatedFlatpakByID(t *testing.T) {
	// A Flatpak entry with only an Id falls back to the id for its name.
	pkgs, err := ParseOutdated([]byte(`{"Packages":[],"Aur":[],"Flatpak":[{"Id":"org.gimp.GIMP","Version":"2.10.38"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 1 || pkgs[0].Name != "flatpak:org.gimp.GIMP" || pkgs[0].Latest != "2.10.38" {
		t.Fatalf("flatpak-by-id: got %v", pkgs)
	}
}

func TestParseOutdatedEmpty(t *testing.T) {
	// Nothing outdated: empty channel arrays (with or without the preamble line)
	// and truly empty output all yield zero packages without erroring.
	for _, in := range []string{
		"Checking for updates...\n{\"MetaData\":{},\"Packages\":[],\"Aur\":[],\"Flatpak\":[]}",
		`{"Packages":[],"Aur":[],"Flatpak":[]}`,
		``,
		"   \n",
	} {
		pkgs, err := ParseOutdated([]byte(in))
		if err != nil {
			t.Fatalf("%q: %v", in, err)
		}
		if len(pkgs) != 0 {
			t.Fatalf("%q: want 0 packages, got %d", in, len(pkgs))
		}
	}
}

func TestParseOutdatedNoJSON(t *testing.T) {
	// Non-empty output with no JSON object (e.g. an error banner) must error
	// rather than silently report "nothing outdated".
	if _, err := ParseOutdated([]byte("error: could not reach mirror")); err == nil {
		t.Error("expected an error when output carries no JSON object")
	}
}

func TestParseOutdatedMalformed(t *testing.T) {
	if _, err := ParseOutdated([]byte("Checking...\n{ this is not json")); err == nil {
		t.Error("expected an error for malformed JSON")
	}
}
