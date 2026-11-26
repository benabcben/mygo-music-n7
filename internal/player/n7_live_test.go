package player

import (
	"os"
	"testing"

	"github.com/egoist/mygo/audio/mp4"
)

// The YouTube AAC stream used by the N7 source can be decoded by MyGo's own player.
func TestN7LiveAACDecode(t *testing.T) {
	path := os.Getenv("MYGO_N7_AAC_SAMPLE")
	if path == "" {
		t.Skip("requires a downloaded audio sample")
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	d, err := mp4.NewDecoder(f)
	if err != nil {
		t.Fatal(err)
	}
	samples := make([]float32, 4096)
	n, err := d.Read(samples)
	if err != nil || n == 0 {
		t.Fatalf("AAC decode: %d samples, %v", n, err)
	}
}
