package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"mygo-music/internal/plugin"
)

// The bundled sources install once and never overwrite an existing source or reappear after deletion.
func TestSeedN7Plugins(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "plugins")
	existing := filepath.Join(dir, "netease")
	if err := os.MkdirAll(existing, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(existing, "custom"), []byte("user"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := seedN7Plugins(dir); err != nil {
		t.Fatal(err)
	}
	sources := plugin.Discover(dir, filepath.Join(root, "plugin-data"))
	if len(sources) != 4 {
		t.Fatalf("expected four new valid sources, got %d", len(sources))
	}
	for _, p := range sources {
		p.Close()
	}
	if b, err := os.ReadFile(filepath.Join(existing, "custom")); err != nil || string(b) != "user" {
		t.Fatalf("existing source changed: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(dir, "qqmusic-qr")); err != nil {
		t.Fatal(err)
	}
	if err := seedN7Plugins(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "qqmusic-qr")); !os.IsNotExist(err) {
		t.Fatal("deleted source was reinstalled")
	}
}

// All clean bundled plugins load and no account storage is embedded.
func TestN7BundlesLoad(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "plugins")
	if err := seedN7Plugins(dir); err != nil {
		t.Fatal(err)
	}
	sources := plugin.Discover(dir, filepath.Join(root, "plugin-data"))
	if len(sources) != 5 {
		t.Fatalf("got %d sources; want 5", len(sources))
	}
	for _, p := range sources {
		p.Close()
	}
	if _, err := os.Stat(filepath.Join(root, "plugin-data")); !os.IsNotExist(err) {
		t.Fatal("account data was unexpectedly installed")
	}
}

// Upgrading a previous install adds Li Zhi without restoring a source the user removed.
func TestSeedN7Migration(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "plugins")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".n7-plugins-installed"), []byte("1"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := seedN7Plugins(dir); err != nil {
		t.Fatal(err)
	}
	sources := plugin.Discover(dir, filepath.Join(root, "data"))
	if len(sources) != 3 {
		t.Fatalf("upgrade did not preserve removed plugins: %d sources", len(sources))
	}
	for _, p := range sources {
		p.Close()
	}
}

// The immediately previous version installs only the newly added sources.
func TestSeedN7V2Migration(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "plugins")
	if err := os.WriteFile(filepath.Join(root, ".n7-plugins-installed-v2"), []byte("1"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := seedN7Plugins(dir); err != nil {
		t.Fatal(err)
	}
	sources := plugin.Discover(dir, filepath.Join(root, "data"))
	if len(sources) != 2 {
		t.Fatalf("got %d new sources, want 2", len(sources))
	}
	for _, p := range sources {
		p.Close()
	}
}

// Empty files from the early Android seed are repaired without replacing custom data.
func TestSeedN7RepairsEmptyPlugins(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "plugins")
	broken := filepath.Join(dir, "netease")
	if err := os.MkdirAll(broken, 0700); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"plugin.json", "index.js"} {
		if err := os.WriteFile(filepath.Join(broken, f), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(broken, "README.md"), []byte("custom"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".n7-plugins-installed-v3"), []byte("1"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := seedN7Plugins(dir); err != nil {
		t.Fatal(err)
	}
	sources := plugin.Discover(dir, filepath.Join(root, "data"))
	if len(sources) != 2 {
		t.Fatalf("repair yielded %d sources", len(sources))
	}
	for _, p := range sources {
		p.Close()
	}
	if b, err := os.ReadFile(filepath.Join(broken, "README.md")); err != nil || string(b) != "custom" {
		t.Fatal("custom file changed")
	}
	if _, err := os.Stat(filepath.Join(dir, "jay-chou")); !os.IsNotExist(err) {
		t.Fatal("deleted plugin was restored")
	}
}

// Presets are applied once, leaving later volume and source choices intact.
func TestCarPresetOnce(t *testing.T) {
	s := settings{Source: "qqmusic", Volume: 0.2}
	if !applyCarPreset(&s) || s.Volume != 1 || s.Source != "qqmusic-qr" {
		t.Fatal("preset was not applied")
	}
	if !slices.Contains(s.DisabledPlugins, "lizhi") {
		t.Fatal("Li Zhi was not disabled")
	}
	s.Volume = 0.3
	s.DisabledPlugins = nil
	if applyCarPreset(&s) || s.Volume != 0.3 || len(s.DisabledPlugins) != 0 {
		t.Fatal("later settings changed")
	}
}

// Updating the installed QR plugin retains its independent account data.
func TestCarQQUpdate(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "qqmusic-qr")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	old := `const isWechat = method === "wechat" || method === "微信" || method === "wx";`
	if err := os.WriteFile(filepath.Join(dir, "index.js"), []byte(old), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(`{"id":"qqmusic-qr","name":"QQ Music 扫码版","settings":[{"key":"loginMethod"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "keep"), []byte("user"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := updateCarQQ(dir); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "index.js"))
	if !strings.Contains(string(b), "const isWechat = true;") {
		t.Fatal("WeChat not selected")
	}
	b, _ = os.ReadFile(filepath.Join(dir, "plugin.json"))
	if !strings.Contains(string(b), "QQ音乐") || strings.Contains(string(b), "loginMethod") {
		t.Fatal("manifest not updated")
	}
	b, _ = os.ReadFile(filepath.Join(dir, "keep"))
	if string(b) != "user" {
		t.Fatal("user data changed")
	}
}
