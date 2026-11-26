package app

import (
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestCarPlaylistSubmenu(t *testing.T) {
	type add struct{ playlist, song string }
	calls := make(chan add, 1)
	a := editTestApp()
	fake(a).addTo = func(id string, songs []string) error { calls <- add{id, songs[0]}; return nil }
	tr := song("2", "宝船", "DOUDOU")
	tt := ui.NewTester(func(c *ui.Context) {
		b := ui.ButtonBase(c.Key("more")).Label("More").Size(48, 48)
		a.carSongMenu(b, &tr)
	}, 640, 800)
	tt.Frame()
	if err := tt.Click("More"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Add to Playlist") {
		t.Fatal("missing playlist entry")
	}
	if tt.HasText("睡前") {
		t.Fatal("playlist expanded before selection")
	}
	if err := tt.Click("Add to Playlist"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("睡前") {
		t.Fatal("playlist submenu did not open")
	}
	if err := tt.Click("睡前"); err != nil {
		t.Fatal(err)
	}
	if got := wait(t, calls, "the adding"); got != (add{"11", "2"}) {
		t.Fatalf("added %+v", got)
	}
}
