//go:build !js

// Command mentions shows a mentions feed in the terminal: where a project is
// talked about, with a fuzzy search, filters and local triage. The feed
// format is in docs/feed.md.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"example.com/mentions-ui/feed"
	"example.com/mentions-ui/internal/demo"
	"example.com/mentions-ui/internal/ui"
)

const usage = `Usage:
  mentions [flags] [FEED]        show a feed (a file or an http(s) URL)
  mentions --demo                show the bundled sample feed
  mentions validate FEED         check a feed against the spec
  mentions publish [flags] FEED  write a public feed with the featured items only

FEED defaults to $MENTIONS_FEED, then ./mentions.json.

Flags:
`

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "validate":
			os.Exit(validate(os.Args[2:]))
		case "publish":
			os.Exit(publish(os.Args[2:]))
		}
	}
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("mentions", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), usage)
		fs.PrintDefaults()
	}
	demoMode := fs.Bool("demo", false, "show the bundled sample feed. Triage stays in memory.")
	statePath := fs.String("state", "", "the triage state file (default: next to a file feed, or in the user state folder for a URL)")
	readOnly := fs.Bool("read-only", false, "keep triage in memory and write nothing")
	ascii := fs.Bool("ascii", os.Getenv("MENTIONS_ASCII") != "", "use ASCII marks in place of Nerd Font glyphs")
	name := fs.String("name", "", "the project name (default: from the feed)")
	subtitle := fs.String("subtitle", "", "the line under the name (default: from the feed)")
	accent := fs.String("accent", "", "the main colour, #rrggbb (default: from the feed)")
	accent2 := fs.String("accent2", "", "the second wordmark colour, #rrggbb (default: from the feed)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	var f *feed.Feed
	var store ui.Store
	note := ""
	if *demoMode {
		f = demo.Feed(time.Now())
		store, note = ui.MemoryStore{}, "demo"
	} else {
		src := fs.Arg(0)
		if src == "" {
			src = os.Getenv("MENTIONS_FEED")
		}
		if src == "" {
			src = "mentions.json"
		}
		b, err := readFeed(src)
		if err != nil {
			fmt.Fprintln(os.Stderr, "mentions:", err)
			fmt.Fprintln(os.Stderr, "Run mentions --demo to see the sample feed.")
			return 1
		}
		f, err = feed.Parse(b)
		if err != nil {
			fmt.Fprintf(os.Stderr, "mentions: %s is not a valid feed:\n%v\n", src, err)
			return 1
		}
		switch {
		case *readOnly:
			store, note = ui.MemoryStore{}, "read only"
		case *statePath != "":
			store = ui.FileStore{Path: *statePath}
		default:
			store = ui.FileStore{Path: defaultState(src, f.Project.Name)}
		}
	}
	if *name != "" {
		f.Project.Name = *name
	}
	if *subtitle != "" {
		f.Project.Subtitle = *subtitle
	}
	ui.SetColors(pickStr(*accent, f.Project.Colors.Accent), pickStr(*accent2, f.Project.Colors.Accent2))

	tri, err := ui.NewTriage(store)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mentions:", err)
		return 1
	}
	ds := ui.NewDataset(f)
	m := ui.New(ds, tri, ui.Options{ASCII: *ascii, Note: note, Open: openURL})
	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "mentions:", err)
		return 1
	}
	return 0
}

func pickStr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// readFeed reads a file, or fetches an http(s) URL with a 20 second limit.
func readFeed(src string) ([]byte, error) {
	if !strings.HasPrefix(src, "http://") && !strings.HasPrefix(src, "https://") {
		b, err := os.ReadFile(src)
		if err != nil {
			return nil, fmt.Errorf("cannot read the feed: %w", err)
		}
		return b, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot fetch the feed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cannot fetch the feed: %s returned %s", src, res.Status)
	}
	return io.ReadAll(io.LimitReader(res.Body, 64<<20))
}

var reUnsafe = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// defaultState puts the state next to a file feed, as <name>.state.json. A
// URL feed keeps its state in the user state folder.
func defaultState(src, project string) string {
	if !strings.Contains(src, "://") {
		base := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
		return filepath.Join(filepath.Dir(src), base+".state.json")
	}
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".local", "state")
	}
	key := reUnsafe.ReplaceAllString(strings.TrimPrefix(strings.TrimPrefix(src, "https://"), "http://"), "_")
	return filepath.Join(dir, "mentions", reUnsafe.ReplaceAllString(project, "_")+"-"+key+".json")
}

func openURL(u string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", u)
		case "windows":
			cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
		default:
			cmd = exec.Command("xdg-open", u)
		}
		if err := cmd.Start(); err != nil {
			return ui.OpenedMsg{Err: err}
		}
		go cmd.Wait()
		return ui.OpenedMsg{}
	}
}

func validate(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "Usage: mentions validate FEED")
		return 2
	}
	b, err := readFeed(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "mentions:", err)
		return 1
	}
	f, err := feed.Parse(b)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s is not valid:\n%v\n", args[0], err)
		return 1
	}
	fmt.Printf("%s is valid: %d items for %s\n", args[0], len(f.Items), f.Project.Name)
	return 0
}

func publish(args []string) int {
	fs := flag.NewFlagSet("publish", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: mentions publish [-state FILE] [-o FILE] FEED")
		fmt.Fprintln(fs.Output(), "Writes a public feed: the featured items only, without relevance, reasons, files or seen times.")
		fs.PrintDefaults()
	}
	statePath := fs.String("state", "", "the triage state file (default: next to the feed)")
	out := fs.String("o", "-", "the output file, or - for standard output")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return 2
	}
	src := fs.Arg(0)
	b, err := readFeed(src)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mentions:", err)
		return 1
	}
	f, err := feed.Parse(b)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s is not valid:\n%v\n", src, err)
		return 1
	}
	sp := *statePath
	if sp == "" {
		sp = defaultState(src, f.Project.Name)
	}
	st, err := ui.FileStore{Path: sp}.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "mentions:", err)
		return 1
	}
	pub := f.Public(func(it *feed.Item) bool {
		s := st.Get(it.ID)
		if s == "" {
			s = it.State
		}
		return s == feed.StateFeatured
	})
	enc, err := pub.Encode()
	if err != nil {
		fmt.Fprintln(os.Stderr, "mentions:", err)
		return 1
	}
	if *out == "-" {
		os.Stdout.Write(enc)
	} else if err := os.WriteFile(*out, enc, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "mentions:", err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "Wrote %d featured items of %d.\n", len(pub.Items), len(f.Items))
	return 0
}
