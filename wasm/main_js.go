//go:build js && wasm

// Command wasm runs the mentions viewer in a browser tab. The page owns the
// terminal renderer and hands the feed text to the program before it starts.
// Triage is kept in the browser's localStorage and nowhere else.
//
// The page API, all on one global object, `mentions`:
//
//	mentions.onOutput(fn)       fn(Uint8Array) gets the bytes for the terminal
//	mentions.input(data)        keyboard and mouse bytes, a string or Uint8Array
//	mentions.resize(cols, rows) the renderer's size in cells
//
// Before go.run, the page sets:
//
//	window.mentionsFeedText     the feed JSON as a string, or "" for the demo
//	window.mentionsInitialSize  [cols, rows]
package main

import (
	"bytes"
	"os"
	"sync"
	"syscall/js"
	"time"

	tea "charm.land/bubbletea/v2"

	"example.com/mentions-ui/feed"
	"example.com/mentions-ui/internal/demo"
	"example.com/mentions-ui/internal/ui"
)

type bridge struct {
	mu      sync.Mutex
	output  js.Value
	pending [][]byte
}

var out = &bridge{}

// Write sends terminal output to the page. There is no pty to turn LF into
// CR LF, so it is done here.
func (b *bridge) Write(p []byte) (int, error) {
	n := len(p)
	q := make([]byte, 0, len(p)+bytes.Count(p, []byte("\n")))
	for i, c := range p {
		if c == '\n' && (i == 0 || p[i-1] != '\r') {
			q = append(q, '\r')
		}
		q = append(q, c)
	}
	b.mu.Lock()
	fn := b.output
	if fn.IsUndefined() || fn.IsNull() {
		b.pending = append(b.pending, q)
		b.mu.Unlock()
		return n, nil
	}
	b.mu.Unlock()
	send(fn, q)
	return n, nil
}

func send(fn js.Value, p []byte) {
	arr := js.Global().Get("Uint8Array").New(len(p))
	js.CopyBytesToJS(arr, p)
	fn.Invoke(arr)
}

// pipe is what Bubble Tea reads the page's input from.
type pipe struct {
	mu   sync.Mutex
	cond *sync.Cond
	buf  []byte
}

func newPipe() *pipe {
	p := &pipe{}
	p.cond = sync.NewCond(&p.mu)
	return p
}

func (p *pipe) write(b []byte) {
	p.mu.Lock()
	p.buf = append(p.buf, b...)
	p.cond.Broadcast()
	p.mu.Unlock()
}

func (p *pipe) Read(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for len(p.buf) == 0 {
		p.cond.Wait()
	}
	n := copy(b, p.buf)
	p.buf = p.buf[n:]
	return n, nil
}

// localStore keeps triage in localStorage under one key. A browser that
// blocks storage gets an empty state, and saves do nothing.
type localStore struct{ key string }

func (s localStore) storage() (v js.Value) {
	defer func() {
		if recover() != nil {
			v = js.Undefined()
		}
	}()
	return js.Global().Get("localStorage")
}

func (s localStore) Load() (st *feed.State, err error) {
	defer func() {
		if recover() != nil {
			st, err = feed.NewState(), nil
		}
	}()
	ls := s.storage()
	if !ls.Truthy() {
		return feed.NewState(), nil
	}
	v := ls.Call("getItem", s.key)
	if v.Type() != js.TypeString {
		return feed.NewState(), nil
	}
	st, err = feed.ParseState([]byte(v.String()))
	if err != nil {
		return feed.NewState(), nil
	}
	return st, nil
}

func (s localStore) Save(st *feed.State) (err error) {
	defer func() {
		if recover() != nil {
			err = nil
		}
	}()
	ls := s.storage()
	if !ls.Truthy() {
		return nil
	}
	b, err := st.Encode()
	if err != nil {
		return err
	}
	ls.Call("setItem", s.key, string(b))
	return nil
}

func main() {
	_ = os.Setenv("TERM", "xterm-256color")
	_ = os.Setenv("COLORTERM", "truecolor")
	_ = os.Setenv("CLICOLOR_FORCE", "1")

	cols, rows := 120, 36
	if v := js.Global().Get("mentionsInitialSize"); v.Truthy() {
		cols, rows = v.Index(0).Int(), v.Index(1).Int()
	}

	var f *feed.Feed
	var store ui.Store
	note := "triage in this browser"
	text := ""
	if v := js.Global().Get("mentionsFeedText"); v.Type() == js.TypeString {
		text = v.String()
	}
	if text != "" {
		parsed, err := feed.Parse([]byte(text))
		if err != nil {
			js.Global().Get("console").Call("error", "mentions: the feed is not valid: "+err.Error())
			text = ""
		} else {
			f = parsed
		}
	}
	if text == "" {
		f = demo.Feed(time.Now())
		store, note = ui.MemoryStore{}, "demo"
	} else {
		store = localStore{key: "mentions-state:" + f.Project.Name}
	}
	ui.SetColors(f.Project.Colors.Accent, f.Project.Colors.Accent2)
	tri, err := ui.NewTriage(store)
	if err != nil {
		tri, _ = ui.NewTriage(ui.MemoryStore{})
	}
	ds := ui.NewDataset(f)
	model := ui.New(ds, tri, ui.Options{
		Note:   note,
		NoQuit: true,
		Open: func(u string) tea.Cmd {
			return func() tea.Msg {
				js.Global().Call("open", u, "_blank", "noopener")
				return ui.OpenedMsg{}
			}
		},
		Copy: func(u string) tea.Cmd {
			return func() tea.Msg {
				if c := js.Global().Get("navigator").Get("clipboard"); c.Truthy() {
					c.Call("writeText", u)
				}
				return nil
			}
		},
	})

	in := newPipe()
	prog := tea.NewProgram(model,
		tea.WithInput(in),
		tea.WithOutput(out),
		tea.WithWindowSize(cols, rows),
	)

	api := js.Global().Get("Object").New()
	api.Set("onOutput", js.FuncOf(func(_ js.Value, args []js.Value) any {
		out.mu.Lock()
		out.output = args[0]
		pending := out.pending
		out.pending = nil
		out.mu.Unlock()
		for _, p := range pending {
			send(args[0], p)
		}
		return nil
	}))
	api.Set("input", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		v := args[0]
		if v.Type() == js.TypeString {
			in.write([]byte(v.String()))
			return nil
		}
		buf := make([]byte, v.Get("length").Int())
		js.CopyBytesToGo(buf, v)
		in.write(buf)
		return nil
	}))
	api.Set("resize", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) < 2 {
			return nil
		}
		c, r := args[0].Int(), args[1].Int()
		if c > 0 && r > 0 {
			// A js.Func must not block, and Send blocks until the loop
			// takes the message.
			go prog.Send(tea.WindowSizeMsg{Width: c, Height: r})
		}
		return nil
	}))
	js.Global().Set("mentions", api)
	if ready := js.Global().Get("onMentionsReady"); ready.Type() == js.TypeFunction {
		ready.Invoke(api)
	}
	if _, err := prog.Run(); err != nil {
		js.Global().Get("console").Call("error", "mentions: "+err.Error())
	}
	if done := js.Global().Get("onMentionsExit"); done.Type() == js.TypeFunction {
		done.Invoke()
	}
}
