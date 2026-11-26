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
