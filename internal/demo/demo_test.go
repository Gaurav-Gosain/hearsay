package demo

import (
	"strings"
	"testing"
	"time"
)

// The sample ships in the binary and must hold only invented data.
func TestSampleIsValidAndInvented(t *testing.T) {
	f := Feed(time.Now())
	if len(f.Items) == 0 {
		t.Fatal("the sample is empty")
	}
	for _, it := range f.Items {
		host := strings.SplitN(strings.TrimPrefix(it.URL, "https://"), "/", 2)[0]
		switch host {
		case "example.com", "example.org", "example.net":
		default:
			t.Errorf("%s links to %s, not an example domain", it.ID, host)
		}
	}
}
