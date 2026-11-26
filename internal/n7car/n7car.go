// Package n7car bridges Android media controls and sign-in web views.
package n7car

import (
	"context"
	"encoding/json"
	"time"

	"github.com/egoist/mygo"
)

func Available() bool { return mygo.Native("n7car").Available() }
func Call(ctx context.Context, method string, args, result any) error {
	return mygo.Native("n7car").Call(ctx, method, args, result)
}
func OnCommand(fn func(string, time.Duration)) {
	mygo.Native("n7car").On("command", func(data []byte) {
		var e struct {
			Name     string `json:"name"`
			Position int64  `json:"position"`
		}
		if json.Unmarshal(data, &e) == nil {
			mygo.RunOnMain(func() { fn(e.Name, time.Duration(e.Position)*time.Millisecond) })
		}
	})
}

// OnVoice registers fn to receive BYD AutoVoice commands beyond basic transport controls.
func OnVoice(fn func(method, params string)) {
	mygo.Native("n7car").On("voice", func(data []byte) {
		var e struct {
			Method string `json:"method"`
			JSON   string `json:"json"`
		}
		if json.Unmarshal(data, &e) == nil && e.Method != "" {
			mygo.RunOnMain(func() { fn(e.Method, e.JSON) })
		}
	})
}
