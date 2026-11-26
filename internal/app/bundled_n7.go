package app

import (
	"crypto/sha256"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed bundled_plugins/youtube-music bundled_plugins/netease bundled_plugins/lizhi bundled_plugins/kugou bundled_plugins/qqmusic-qr
var n7Plugins embed.FS

// seedN7Plugins installs the supplied sources once without overwriting a user's plugins.
func seedN7Plugins(dir string) error {
	if err := upgradeBundledYouTube(dir); err != nil {
		return err
	}
	marker := filepath.Join(filepath.Dir(dir), ".n7-plugins-installed-v6")
	if _, err := os.Stat(marker); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	names := []string{"youtube-music", "netease", "lizhi", "kugou", "qqmusic-qr"}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), ".n7-plugins-installed-v5")); err == nil {
		names = nil
	} else if !os.IsNotExist(err) {
		return err
	} else if _, err := os.Stat(filepath.Join(filepath.Dir(dir), ".n7-plugins-installed-v4")); err == nil {
		names = []string{"qqmusic-qr"}
	} else if !os.IsNotExist(err) {
		return err
	} else if _, err := os.Stat(filepath.Join(filepath.Dir(dir), ".n7-plugins-installed-v3")); err == nil {
		names = []string{"qqmusic-qr"}
	} else if !os.IsNotExist(err) {
		return err
	} else if _, err := os.Stat(filepath.Join(filepath.Dir(dir), ".n7-plugins-installed-v2")); err == nil {
		names = []string{"kugou", "qqmusic-qr"}
	} else if !os.IsNotExist(err) {
		return err
	} else if _, err := os.Stat(filepath.Join(filepath.Dir(dir), ".n7-plugins-installed")); err == nil {
		names = []string{"lizhi", "kugou", "qqmusic-qr"}
	} else if !os.IsNotExist(err) {
		return err
	}
	for _, name := range names {
		dst := filepath.Join(dir, name)
		if _, err := os.Stat(dst); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return err
		}
		tmp, err := os.MkdirTemp(dir, ".n7-plugin-")
		if err != nil {
			return err
		}
		err = fs.WalkDir(n7Plugins, "bundled_plugins/"+name, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel := strings.TrimPrefix(path, "bundled_plugins/"+name)
			out := filepath.Join(tmp, rel)
			if d.IsDir() {
				return os.MkdirAll(out, 0700)
			}
			data, err := n7Plugins.ReadFile(path)
			if err != nil {
				return err
			}
			return os.WriteFile(out, data, 0600)
		})
		if err == nil {
			err = os.Rename(tmp, dst)
		}
		if err != nil {
			os.RemoveAll(tmp)
			return err
		}
	}
	// Repair a broken early Android seed that wrote empty plugin files. Preserve
	// deleted sources, custom files and all nonempty user changes.
	for _, name := range []string{"youtube-music", "netease", "lizhi", "kugou", "qqmusic-qr"} {
		dst := filepath.Join(dir, name)
		manifest, e1 := os.Stat(filepath.Join(dst, "plugin.json"))
		main, e2 := os.Stat(filepath.Join(dst, "index.js"))
		if e1 != nil || e2 != nil || manifest.Size() != 0 || main.Size() != 0 {
			continue
		}
		if err := fs.WalkDir(n7Plugins, "bundled_plugins/"+name, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			out := filepath.Join(dst, strings.TrimPrefix(path, "bundled_plugins/"+name))
			if d.IsDir() {
				return os.MkdirAll(out, 0700)
			}
			info, err := os.Stat(out)
			if err == nil && info.Size() != 0 {
				return nil
			}
			if err != nil && !os.IsNotExist(err) {
				return err
			}
			data, err := n7Plugins.ReadFile(path)
			if err != nil {
				return err
			}
			return os.WriteFile(out, data, 0600)
		}); err != nil {
			return err
		}
	}
	if err := updateCarQQ(filepath.Join(dir, "qqmusic-qr")); err != nil {
		return err
	}
	return os.WriteFile(marker, []byte("1\n"), 0600)
}

// Upgrade only the exact older bundled source, leaving user-modified plugins
// and the separate account/settings data alone.
func upgradeBundledYouTube(dir string) error {
	main := filepath.Join(dir, "youtube-music", "index.js")
	data, err := os.ReadFile(main)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if fmt.Sprintf("%x", sha256.Sum256(data)) != "8b702e3feeb3b0d0076168b2b79ae5ca650dfa1a69bc09dd957b92321bf5fe00" {
		return nil
	}
	manifest := filepath.Join(dir, "youtube-music", "plugin.json")
	raw, err := os.ReadFile(manifest)
	if err != nil {
		return err
	}
	var installed, bundled map[string]json.RawMessage
	if err = json.Unmarshal(raw, &installed); err != nil {
		return err
	}
	newManifest, err := n7Plugins.ReadFile("bundled_plugins/youtube-music/plugin.json")
	if err != nil {
		return err
	}
	if err = json.Unmarshal(newManifest, &bundled); err != nil {
		return err
	}
	// Only migrate the known unmodified manifest. A custom manifest may define
	// its own permissions or settings, and must not be silently replaced.
	var id string
	if err = json.Unmarshal(installed["id"], &id); err != nil || id != "youtube-music" {
		return nil
	}
	var oldPerm, newPerm map[string]json.RawMessage
	if err = json.Unmarshal(installed["permissions"], &oldPerm); err != nil {
		return err
	}
	if err = json.Unmarshal(bundled["permissions"], &newPerm); err != nil {
		return err
	}
	var oldHosts, newHosts []string
	if err = json.Unmarshal(oldPerm["network"], &oldHosts); err != nil {
		return err
	}
	if err = json.Unmarshal(newPerm["network"], &newHosts); err != nil {
		return err
	}
	for _, host := range newHosts {
		found := false
		for _, existing := range oldHosts {
			if existing == host {
				found = true
				break
			}
		}
		if !found {
			oldHosts = append(oldHosts, host)
		}
	}
	oldPerm["network"], err = json.Marshal(oldHosts)
	if err != nil {
		return err
	}
	installed["permissions"], err = json.Marshal(oldPerm)
	if err != nil {
		return err
	}
	var oldSettings, newSettings []map[string]any
	if b := installed["settings"]; len(b) > 0 {
		if err = json.Unmarshal(b, &oldSettings); err != nil {
			return err
		}
	}
	if err = json.Unmarshal(bundled["settings"], &newSettings); err != nil {
		return err
	}
	for _, setting := range newSettings {
		found := false
		for _, existing := range oldSettings {
			if existing["key"] == setting["key"] {
				found = true
				break
			}
		}
		if !found {
			oldSettings = append(oldSettings, setting)
		}
	}
	installed["settings"], err = json.Marshal(oldSettings)
	if err != nil {
		return err
	}
	raw, err = json.MarshalIndent(installed, "", "  ")
	if err != nil {
		return err
	}
	if err = atomicCarWrite(manifest, append(raw, '\n')); err != nil {
		return err
	}
	content, err := n7Plugins.ReadFile("bundled_plugins/youtube-music/index.js")
	if err != nil {
		return err
	}
	return atomicCarWrite(main, content)
}

// Update the installed bundled QR plugin in place without touching its account storage.
func updateCarQQ(dir string) error {
	main := filepath.Join(dir, "index.js")
	data, err := os.ReadFile(main)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	old := "const isWechat = method === \"wechat\" || method === \"\u5fae\u4fe1\" || method === \"wx\";"
	if strings.Contains(string(data), old) {
		next := strings.Replace(string(data), old, `const isWechat = true;`, 1)
		if err := atomicCarWrite(main, []byte(next)); err != nil {
			return err
		}
	}
	manifest := filepath.Join(dir, "plugin.json")
	b, err := os.ReadFile(manifest)
	if err != nil {
		return err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	if m["id"] != "qqmusic-qr" {
		return nil
	}
	m["name"] = "QQ\u97f3\u4e50"
	delete(m, "settings")
	b, err = json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return atomicCarWrite(manifest, append(b, '\n'))
}
func atomicCarWrite(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".n7-update-")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(data)
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, path)
}
