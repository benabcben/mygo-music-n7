package app

import (
	"testing"

	"mygo-music/internal/music"
)

// TestBYDVoiceSearchQuery checks that AutoVoice search payloads combine artist and song or fall back cleanly.
func TestBYDVoiceSearchQuery(t *testing.T) {
	cases := []struct {
		json string
		want string
	}{
		{`{"artist":"周杰伦","song":"晴天"}`, "周杰伦 晴天"},
		{`{"EXTRA_ARTIST_SEARCH":"周杰伦","EXTRA_KEYWORDS_SEARCH":"晴天"}`, "周杰伦 晴天"},
		{`{"EXTRA_KEYWORDS_SEARCH":"晴天"}`, "晴天"},
		{`{"EXTRA_ARTIST_SEARCH":"周杰伦"}`, "周杰伦"},
		{`{"artist":"周杰伦","song":"周杰伦 晴天"}`, "周杰伦 晴天"},
		{`{"keyword":"稻香"}`, "稻香"},
		{`{"singer":"陈奕迅"}`, "陈奕迅"},
		{`{}`, ""},
	}
	for _, tc := range cases {
		if got := bydVoiceSearchQuery(tc.json); got != tc.want {
			t.Errorf("bydVoiceSearchQuery(%s) = %q, want %q", tc.json, got, tc.want)
		}
	}
}

// TestVoiceSearchAndPlay checks that a searchMusic voice command queries the active source and starts the top match.
func TestVoiceSearchAndPlay(t *testing.T) {
	a := newTestApp()
	ch := posts(a)
	src := fake(a)
	var gotQuery string
	track := music.Track{ID: "t1", Source: "fake", Name: "晴天"}
	src.search = func(q string, kind music.SearchKind, offset int) (*music.SearchResult, error) {
		gotQuery = q
		return &music.SearchResult{Tracks: []music.Track{track}}, nil
	}
	a.handleVoice("searchMusic", `{"artist":"周杰伦","song":"晴天"}`)
	until(t, ch, func() bool { return a.pl.Current() != nil })
	if gotQuery != "周杰伦 晴天" {
		t.Fatalf("search query = %q, want %q", gotQuery, "周杰伦 晴天")
	}
	if cur := a.pl.Current(); cur == nil || cur.ID != track.ID {
		t.Fatalf("current track = %#v, want %q", cur, track.ID)
	}
}

// TestVoicePlayRandomFallback checks that playRandom loads recent tracks when the queue is empty.
func TestVoicePlayRandomFallback(t *testing.T) {
	a := newTestApp()
	ch := posts(a)
	src := fake(a)
	src.recent = func() ([]music.Track, error) {
		return []music.Track{{ID: "r1", Source: "fake", Name: "Song A"}}, nil
	}
	a.handleVoice("playRandom", `{"mediaType":1001}`)
	until(t, ch, func() bool { return a.pl.Current() != nil })
	if cur := a.pl.Current(); cur == nil || cur.ID != "r1" {
		t.Fatalf("current track = %#v, want r1", cur)
	}
}
