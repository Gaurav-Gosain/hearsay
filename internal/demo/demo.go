// Package demo holds the made-up feed behind --demo. gen/main.go writes it.
// Every name and link in it is invented.
package demo

import (
	_ "embed"
	"time"

	"example.com/mentions-ui/feed"
)

//go:embed sample.json
var sample []byte

// Feed returns the sample with every date moved so the sample's own "now"
// is the real now, so the list reads "today", "2d" and so on.
func Feed(now time.Time) *feed.Feed {
	f, err := feed.Parse(sample)
	if err != nil {
		panic("demo: the bundled sample is not valid: " + err.Error())
	}
	shift := now.Sub(feed.ParseTime(f.Generated))
	move := func(s string) string {
		t := feed.ParseTime(s)
		if t.IsZero() {
			return s
		}
		return t.Add(shift).UTC().Format(time.RFC3339)
	}
	f.Generated = move(f.Generated)
	for i := range f.Items {
		f.Items[i].Published = move(f.Items[i].Published)
	}
	return f
}
