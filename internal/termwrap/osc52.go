package termwrap

import (
	"bytes"
	"encoding/base64"
	"io"
	"log/slog"
	"strings"
)

// ClipboardWrite is one OSC 52 clipboard write lifted out of a session's
// output before the emulator could drop it (#212).
type ClipboardWrite struct {
	// Selection is the OSC 52 selection byte: 'c' for the system clipboard,
	// 'p' for the primary selection. An OSC 52 naming none is 'c'.
	Selection byte
	// Text is the decoded payload, as the child meant it to be pasted. The
	// host re-encodes it, so no base64 travels past this struct.
	Text string
}

// clipQueue is how many lifted writes wait for the Update goroutine. A copy
// is an operator action, so the queue only has to absorb a burst.
const clipQueue = 8

// clipChunk is how much the reader takes from the PTY at a time, matching
// the emulator's own read size.
const clipChunk = 4096

// maxClipPayload bounds one OSC 52 payload. A child that opens a clipboard
// write and never terminates it must not make this reader buffer without
// limit - the PTY read loop is behind it, and a reader that stops returning
// bytes freezes the pane (issue #33). Past the cap the write is dropped,
// which is what the emulator did with every OSC 52 before this existed.
const maxClipPayload = 1 << 20

// clipIntroducer is what follows ESC] in the sequence this reader lifts.
const clipIntroducer = "52;"

// clipLift sits between the PTY and c1Guard and pulls OSC 52 clipboard
// writes out of the stream, so the host terminal can be told about a copy
// the child made (#212).
//
// The emulator under bubbleterm handles OSC 0/1/2, 7, 8, 10-12 and 110-112
// and nothing else, and x/vt's Callbacks carries no clipboard hook, so an
// OSC 52 reaching it is dropped with "unhandled sequence" and the operator's
// clipboard never hears about claude's copy, tmux's set-clipboard, or
// neovim's clipboard provider. Lifting it here is the route #192 opened by
// giving termwrap ownership of the bytes.
//
// Order matters: this reader must sit BEFORE c1Guard, because c1Guard
// rewrites 0x9C inside a string payload to '?', and 0x9C is one of the three
// terminators an OSC 52 may use. Nothing c1Guard does can corrupt a payload
// that has already been lifted out, since base64 is ASCII.
//
// What it does not do, and why:
//   - A '?' payload is the read direction - the child asking the host for
//     the clipboard. It is dropped, exactly as the emulator dropped it.
//     Answering it needs a pending request per session to route the host's
//     reply back into the right PTY, and it would hand anything running in a
//     pane a read of the operator's clipboard. Its own issue if it is wanted.
//   - Nothing gates the write. Every terminal forwards OSC 52 writes and
//     tmux's set-clipboard is the same bridge; a per-copy confirmation would
//     make claude's copy affordance useless.
type clipLift struct {
	src     io.Reader
	writes  chan ClipboardWrite
	scratch []byte
	out     bytes.Buffer // bytes bound for the emulator
	held    []byte       // an introducer, released if this is not an OSC 52
	body    []byte       // the OSC 52 payload as it accumulates
	state   clipState
	err     error // sticky: returned once out is drained
	over    bool  // this payload passed the cap, so it will be dropped
}

type clipState int

const (
	clipGround    clipState = iota
	clipEscape              // an ESC was the last byte
	clipIntro               // inside ESC], still matching "52;"
	clipPayload             // inside an OSC 52 payload
	clipPayloadST           // an ESC inside an OSC 52 payload
	clipOther               // inside some other OSC or DCS payload
)

// newClipLift wraps src. The writes channel is buffered and never blocks the
// caller; see deliver.
func newClipLift(src io.Reader) *clipLift {
	return &clipLift{
		src:     src,
		writes:  make(chan ClipboardWrite, clipQueue),
		scratch: make([]byte, clipChunk),
	}
}

// Writes is the stream of clipboard writes lifted out of the child's output.
func (c *clipLift) Writes() <-chan ClipboardWrite { return c.writes }

// Read fills p with the bytes the emulator should see, which is everything
// the child wrote except the OSC 52 sequences. It loops rather than return
// zero bytes: a read that was entirely one clipboard sequence has nothing to
// hand on yet, and (0, nil) would be read as a stall.
func (c *clipLift) Read(p []byte) (int, error) {
	for c.out.Len() == 0 && c.err == nil {
		n, err := c.src.Read(c.scratch)
		for _, b := range c.scratch[:n] {
			c.scan(b)
		}
		if err != nil {
			c.err = err
			c.release() // a half-typed introducer was never a copy
		}
	}
	if c.out.Len() == 0 {
		return 0, c.err
	}
	n, _ := c.out.Read(p)
	return n, nil
}

// scan advances the state by one byte.
func (c *clipLift) scan(b byte) {
	switch c.state {
	case clipEscape:
		c.afterEscape(b)
	case clipIntro:
		c.matchIntro(b)
	case clipPayload:
		c.inPayload(b)
	case clipPayloadST:
		c.afterPayloadESC(b)
	case clipOther:
		c.inOther(b)
	default:
		c.inGround(b)
	}
}

// inGround passes ordinary output through, holding an ESC on the chance that
// it opens the sequence this reader is looking for.
func (c *clipLift) inGround(b byte) {
	if b == byteESC {
		c.hold(b)
		c.state = clipEscape
		return
	}
	c.out.WriteByte(b)
}

// afterEscape is the byte after ESC: ] may open the OSC 52 being looked for,
// so the introducer stays held; P opens a DCS; anything else is a sequence
// this reader has no business in.
func (c *clipLift) afterEscape(b byte) {
	switch b {
	case ']':
		c.hold(b)
		c.state = clipIntro
	case byteESC:
		c.release()
		c.hold(b)
	case 'P':
		c.releaseWith(b)
		c.state = clipOther
	default:
		c.releaseWith(b)
		c.state = clipGround
	}
}

// matchIntro consumes the bytes after ESC] while they still spell "52;". A
// divergence means some other OSC - a title, a hyperlink - so the held bytes
// are given up and the rest of that payload passes through untouched.
func (c *clipLift) matchIntro(b byte) {
	i := len(c.held) - 2
	if b != clipIntroducer[i] {
		c.releaseWith(b)
		c.state = clipOther
		return
	}
	c.hold(b)
	if i == len(clipIntroducer)-1 {
		c.held, c.body, c.over = c.held[:0], c.body[:0], false
		c.state = clipPayload
	}
}

// inPayload accumulates the payload. BEL and the 8-bit ST finish it, CAN and
// SUB abort it as the parser's table says, and ESC is either the two-byte ST
// or an abort - the next byte decides.
//
// 0x9C is honoured here and deliberately ignored in inOther: these bytes are
// lifted out before c1Guard, so this is the real terminator, while a 0x9C in
// any other payload is about to be rewritten to '?' downstream (#192) and
// honouring it would put this reader out of step with the emulator.
func (c *clipLift) inPayload(b byte) {
	switch b {
	case byteBEL, byteST:
		c.deliver()
		c.state = clipGround
	case byteCAN, byteSUB:
		c.state = clipGround
	case byteESC:
		c.state = clipPayloadST
	default:
		c.appendBody(b)
	}
}

// appendBody grows the payload up to the cap and then stops, remembering
// that it did. Scanning continues either way, so the terminator is still
// found and the stream after it is not mistaken for payload.
func (c *clipLift) appendBody(b byte) {
	if len(c.body) >= maxClipPayload {
		c.over = true
		return
	}
	c.body = append(c.body, b)
}

// afterPayloadESC decides what an ESC inside a payload was: ESC \ is the
// string terminator and finishes the write; anything else aborted the
// sequence, and that byte is scanned again from ground so an ESC opening the
// next sequence is not lost.
func (c *clipLift) afterPayloadESC(b byte) {
	c.state = clipGround
	if b == '\\' {
		c.deliver()
		return
	}
	c.scan(b)
}

// inOther passes another OSC or DCS payload through byte for byte.
func (c *clipLift) inOther(b byte) {
	if b == byteESC {
		c.hold(b)
		c.state = clipEscape
		return
	}
	if b == byteBEL || b == byteCAN || b == byteSUB {
		c.state = clipGround
	}
	c.out.WriteByte(b)
}

// hold keeps a byte back while it may still turn out to introduce an OSC 52.
func (c *clipLift) hold(b byte) { c.held = append(c.held, b) }

// release gives up the held introducer: it was not an OSC 52 after all, so
// the bytes belong to the emulator like any others.
func (c *clipLift) release() {
	c.out.Write(c.held)
	c.held = c.held[:0]
}

func (c *clipLift) releaseWith(b byte) {
	c.release()
	c.out.WriteByte(b)
}

// deliver queues a finished payload for the ui. The send is non-blocking:
// the PTY read loop is on this goroutine, and a full queue must cost a copy
// rather than the pane (issue #33).
func (c *clipLift) deliver() {
	sel, text, ok := c.parseBody()
	if !ok {
		return
	}
	select {
	case c.writes <- ClipboardWrite{Selection: sel, Text: text}:
	default:
		slog.Warn("dropping an OSC 52 clipboard write: the queue is full",
			"queue", clipQueue, "bytes", len(text))
	}
}

// parseBody splits the payload's "<selection>;<base64>" and decodes it,
// reporting whether anything should reach the host at all.
func (c *clipLift) parseBody() (byte, string, bool) {
	if c.over {
		slog.Warn("dropping an OSC 52 clipboard write past the payload cap",
			"cap", maxClipPayload)
		return 0, "", false
	}
	sel, encoded, found := strings.Cut(string(c.body), ";")
	if !found || encoded == "?" {
		return 0, "", false // no selection field, or the read direction
	}
	text, err := decodeClip(encoded)
	if err != nil {
		slog.Warn("dropping an OSC 52 clipboard write that is not base64",
			"bytes", len(encoded), "err", err)
		return 0, "", false
	}
	return selectionOf(sel), text, true
}

// selectionOf is the clipboard the child named. OSC 52 takes a set of
// selection characters and the first is the one that matters here; naming
// none means the system clipboard.
func selectionOf(sel string) byte {
	if sel == "" {
		return 'c'
	}
	return sel[0]
}

// decodeClip accepts both spellings of base64: tmux and neovim pad their
// payloads, and not every writer does.
func decodeClip(encoded string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		b, err = base64.RawStdEncoding.DecodeString(encoded)
	}
	return string(b), err
}
