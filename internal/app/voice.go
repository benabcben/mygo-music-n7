package app

import (
	"encoding/json"
	"strings"

	"mygo-music/internal/music"
	"mygo-music/internal/n7car"
	"mygo-music/internal/player"
)

func (a *app) initVoice() {
	n7car.OnVoice(func(method, params string) {
		a.handleVoice(method, params)
	})
}

// bydVoiceSearchQuery extracts a single search query string from a BYD AutoVoice
// operation payload such as {"artist":"周杰伦","song":"晴天"} or {"keyword":"晴天"}.
func bydVoiceSearchQuery(rawJSON string) string {
	var m map[string]any
	if err := json.Unmarshal([]byte(rawJSON), &m); err != nil {
		return ""
	}
	pick := func(keys ...string) string {
		for _, k := range keys {
			if s, ok := m[k].(string); ok {
				if s = strings.TrimSpace(s); s != "" {
					return s
				}
			}
		}
		return ""
	}
	song := pick("song", "songName", "name", "EXTRA_KEYWORDS_SEARCH")
	artist := pick("artist", "singer", "EXTRA_ARTIST_SEARCH")
	if song != "" {
		if artist != "" && !strings.Contains(strings.ToLower(song), strings.ToLower(artist)) {
			return artist + " " + song
		}
		return song
	}
	if kw := pick("keyword", "keyWord", "query"); kw != "" {
		return kw
	}
	if artist != "" {
		return artist
	}
	return pick("album", "EXTRA_GENRE_SEARCH", "EXTRA_PROGRAM_TAGS", "EXTRA_LANG_SEARCH", "EXTRA_AREA_SEARCH")
}

func (a *app) handleVoice(method, rawJSON string) {
	switch strings.ToLower(strings.TrimSpace(method)) {
	case "play", "resume", "continueplay":
		if a.pl.Current() != nil {
			if !a.pl.Playing() {
				a.pl.TogglePlay()
			}
			a.post(func() {})
			return
		}
		a.playVoiceFallback(false)
	case "playrandom":
		// "播放音乐" arrives as RANDOM_SEARCH; keep the current list and order.
		if a.pl.Current() != nil {
			if !a.pl.Playing() {
				a.pl.TogglePlay()
			}
			a.post(func() {})
			return
		}
		if tracks := a.pl.Queue(); len(tracks) > 0 {
			a.pl.PlayQueue(tracks, 0)
			a.post(func() {})
			return
		}
		a.playVoiceFallback(false)
	case "playrecommend", "playfavorite", "playlately", "playguessyoulike", "playplaylist":
		a.playVoiceFallback(false)
	case "searchmusic":
		q := bydVoiceSearchQuery(rawJSON)
		if q == "" {
			a.playVoiceFallback(false)
			return
		}
		a.playVoiceSearch(q)
	case "pause", "stop":
		if a.pl.Playing() {
			a.pl.TogglePlay()
		}
		a.post(func() {})
	case "next", "playnext":
		a.pl.Next()
		a.post(func() {})
	case "previous", "prev", "pre", "playprev", "playprevious":
		a.pl.Prev()
		a.post(func() {})
	case "setplaypattern":
		a.applyVoicePattern(rawJSON)
	}
}

func (a *app) applyVoicePattern(rawJSON string) {
	low := strings.ToLower(rawJSON)
	switch {
	case strings.Contains(low, "random") || strings.Contains(low, "shuffle"):
		if !a.pl.Shuffle() {
			a.pl.ToggleShuffle()
		}
	case strings.Contains(low, "single"):
		for a.pl.Repeat() != player.RepeatOne {
			a.pl.CycleRepeat()
		}
	case strings.Contains(low, "cycle") || strings.Contains(low, "loop") || strings.Contains(low, "list"):
		if a.pl.Shuffle() {
			a.pl.ToggleShuffle()
		}
		for a.pl.Repeat() != player.RepeatAll {
			a.pl.CycleRepeat()
		}
	case strings.Contains(low, "order"):
		if a.pl.Shuffle() {
			a.pl.ToggleShuffle()
		}
		for a.pl.Repeat() != player.RepeatOff {
			a.pl.CycleRepeat()
		}
	}
	a.post(func() {})
}

func (a *app) voiceLibraries() []*library {
	var out []*library
	seen := map[*library]bool{}
	add := func(lib *library) {
		if lib != nil && !seen[lib] {
			seen[lib] = true
			out = append(out, lib)
		}
	}
	add(a.library)
	for _, lib := range a.libs {
		add(lib)
	}
	return out
}

func (a *app) playVoiceSearch(query string) {
	libs := a.voiceLibraries()
	go func() {
		for _, lib := range libs {
			if !lib.caps["search"] {
				continue
			}
			res, err := answered(lib.src.Search(query, music.SearchTracks, 0))
			if err == nil && res != nil && len(res.Tracks) > 0 {
				tracks := res.Tracks
				a.post(func() { a.pl.PlayQueue(tracks, 0) })
				return
			}
		}
	}()
}

func (a *app) playVoiceFallback(shuffle bool) {
	libs := a.voiceLibraries()
	go func() {
		for _, lib := range libs {
			if tracks := firstPlayableTracks(lib); len(tracks) > 0 {
				a.post(func() {
					if shuffle {
						a.pl.PlayShuffled(tracks)
					} else {
						a.pl.PlayQueue(tracks, 0)
					}
				})
				return
			}
		}
	}()
}

func firstPlayableTracks(lib *library) []music.Track {
	if lib.caps["recent"] {
		if tracks, err := lib.src.Recent(); err == nil && len(tracks) > 0 {
			return tracks
		}
	}
	if lib.caps["liked"] {
		if page, err := lib.src.Liked(0); err == nil && page != nil && len(page.Tracks) > 0 {
			return page.Tracks
		}
	}
	if lib.caps["home"] {
		if home, err := lib.src.Home(); err == nil && home != nil {
			for _, sec := range home.Sections {
				if len(sec.Tracks) > 0 {
					return sec.Tracks
				}
			}
		}
	}
	return nil
}
