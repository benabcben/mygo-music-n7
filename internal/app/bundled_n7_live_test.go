package app

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hajimehoshi/go-mp3"

	"mygo-music/internal/music"
	"mygo-music/internal/plugin"
)

// The bundled plugins return a directly playable AAC stream and timed lyrics with the real plugin runtime.
func TestN7LiveYouTube(t *testing.T) {
	if os.Getenv("MYGO_N7_LIVE_TEST") != "1" {
		t.Skip("requires external services")
	}
	root := t.TempDir()
	dir := filepath.Join(root, "plugins")
	if err := seedN7Plugins(dir); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"youtube-music"} {
		t.Run(name, func(t *testing.T) {
			p, err := plugin.Load(filepath.Join(dir, name), filepath.Join(root, "data"))
			if err != nil {
				t.Fatal(err)
			}
			defer p.Close()
			id := "aJOTlE1K90k"
			if name == "jay-chou" {
				id = "-biOGdYiF-I"
			}
			stream, err := p.Stream(id, "high")
			if err != nil {
				t.Fatal(err)
			}
			if stream.Type != "m4a" || stream.URL == "" || stream.Size == 0 {
				t.Fatalf("invalid AAC stream metadata: type %q size %d", stream.Type, stream.Size)
			}
			lyrics, err := p.Lyrics(id)
			if err != nil {
				t.Fatal(err)
			}
			if lyrics.LRC == "" && len(lyrics.Lines) == 0 {
				t.Fatal("missing timed lyrics")
			}
		})
	}
}

// The preinstalled NetEase source can search and obtain lyrics without copying an account.
func TestN7LiveNetEase(t *testing.T) {
	if os.Getenv("MYGO_N7_LIVE_TEST") != "1" {
		t.Skip("requires external services")
	}
	root := t.TempDir()
	dir := filepath.Join(root, "plugins")
	if err := seedN7Plugins(dir); err != nil {
		t.Fatal(err)
	}
	p, err := plugin.Load(filepath.Join(dir, "netease"), filepath.Join(root, "data"))
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	result, err := p.Search("陈奕迅 十年", music.SearchTracks, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Tracks) == 0 {
		t.Fatal("no search results")
	}
	words, err := p.Lyrics(result.Tracks[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if words.LRC == "" && len(words.Lines) == 0 {
		t.Fatal("no lyrics")
	}
	stream, err := p.Stream(result.Tracks[0].ID, "standard")
	if err != nil {
		t.Logf("anonymous stream unavailable: %v", err)
		return
	}
	if stream.URL == "" {
		t.Fatal("empty stream URL")
	}
	t.Logf("anonymous stream type: %s", stream.Type)
}

// Checks real search, timed lyrics and an available stream without an account.
func TestN7LiveQQAndKugou(t *testing.T) {
	if os.Getenv("MYGO_N7_LIVE_TEST") != "1" {
		t.Skip("requires external services")
	}
	root := t.TempDir()
	dir := filepath.Join(root, "plugins")
	if err := seedN7Plugins(dir); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"kugou", "qqmusic-qr"} {
		t.Run(name, func(t *testing.T) {
			p, err := plugin.Load(filepath.Join(dir, name), filepath.Join(root, "data"))
			if err != nil {
				t.Fatal(err)
			}
			defer p.Close()
			result, err := p.Search("陈奕迅 十年", music.SearchTracks, 0)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Tracks) == 0 {
				t.Fatal("no search results")
			}
			for i, track := range result.Tracks {
				if i >= 5 {
					break
				}
				words, lyricErr := p.Lyrics(track.ID)
				stream, streamErr := p.Stream(track.ID, "standard")
				timed := lyricErr == nil && (words.LRC != "" || len(words.Lines) > 0)
				if streamErr == nil && stream.URL != "" && timed {
					client := &http.Client{Timeout: 20 * time.Second}
					resp, err := client.Get(stream.URL)
					if err != nil {
						t.Logf("audio request: %v", err)
						continue
					}
					if resp.StatusCode != http.StatusOK {
						resp.Body.Close()
						t.Logf("audio status: %s", resp.Status)
						continue
					}
					dec, err := mp3.NewDecoder(resp.Body)
					if err != nil {
						resp.Body.Close()
						t.Logf("audio decoder: %v", err)
						continue
					}
					pcm := make([]byte, 4096)
					n, err := dec.Read(pcm)
					resp.Body.Close()
					if n == 0 {
						t.Logf("audio data: %v", err)
						continue
					}
					t.Logf("search, stream and lyrics available: %s, type %s", track.Name, stream.Type)
					return
				}
				t.Logf("candidate %d: stream=%v lyrics=%v timed=%v", i, streamErr, lyricErr, timed)
			}
			t.Fatal("no candidate returned both a stream and lyrics")
		})
	}
}

// QQ QR source can search and display its actual login code without an account.
func TestN7LiveQQQR(t *testing.T) {
	if os.Getenv("MYGO_N7_LIVE_TEST") != "1" {
		t.Skip("requires external services")
	}
	root := t.TempDir()
	dir := filepath.Join(root, "plugins")
	if err := seedN7Plugins(dir); err != nil {
		t.Fatal(err)
	}
	p, err := plugin.Load(filepath.Join(dir, "qqmusic-qr"), filepath.Join(root, "data"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := p.Search("陈奕迅 十年", music.SearchTracks, 0)
	if err != nil || len(result.Tracks) == 0 {
		t.Fatalf("search: %v", err)
	}
	qr := make(chan string, 1)
	done := make(chan error, 1)
	p.OnSignInPrompt(func(prompt music.SignInPrompt) {
		if prompt.QR != "" {
			select {
			case qr <- prompt.QR:
			default:
			}
		}
	})
	go func() { _, e := p.SignIn(); done <- e }()
	select {
	case code := <-qr:
		if code == "" {
			t.Fatal("empty QR")
		}
		t.Log("QR code generated")
	case err := <-done:
		t.Fatalf("sign-in ended before QR: %v", err)
	case <-time.After(30 * time.Second):
		t.Fatal("QR generation timed out")
	}
	p.Close()
}
