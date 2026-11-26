//go:build android

package player

import (
	"context"
	"time"

	"github.com/egoist/mygo"
	"mygo-music/internal/n7car"
)

type n7MediaState struct {
	Showing  bool   `json:"showing"`
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	Album    string `json:"album"`
	Artwork  []byte `json:"artwork"`
	Duration int64  `json:"duration"`
	Position int64  `json:"position"`
	Playing  bool   `json:"playing"`
}

var n7Updates = make(chan n7MediaState, 1)

func initN7Media(p *Player) {
	n7car.OnCommand(func(name string, pos time.Duration) {
		switch name {
		case "play":
			if !p.playing {
				p.TogglePlay()
			}
		case "pause":
			if p.playing {
				p.TogglePlay()
			}
		case "toggle":
			p.TogglePlay()
		case "next":
			p.Next()
		case "previous":
			p.Prev()
		case "seek":
			p.Seek(pos)
		}
		p.post(func() {})
	})
	go func() {
		for state := range n7Updates {
			_ = n7car.Call(context.Background(), "media", state, nil)
		}
	}()
}
func updateN7Media() {
	info, ok := mygo.NowPlaying.Info()
	state := n7MediaState{Showing: ok, Title: info.Title, Artist: info.Artist, Album: info.Album, Artwork: info.Artwork, Duration: info.Duration.Milliseconds(), Position: info.Position.Milliseconds(), Playing: info.State == mygo.PlaybackPlaying}
	select {
	case n7Updates <- state:
	default:
		select {
		case <-n7Updates:
		default:
		}
		n7Updates <- state
	}
}
