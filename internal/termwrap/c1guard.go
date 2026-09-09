package termwrap

import "io"

// c1Guard sits between the PTY and the emulator and keeps the byte 0x9C out
// of OSC and DCS payloads (#192).
//
// x/ansi's parser honours 0x9C as the 8-bit C1 string terminator inside an
// OSC or DCS payload even while the stream is UTF-8, where 0x9C is a
// continuation byte (x/ansi@v0.11.8/parser/transition_table.go:269 for OSC,
// :220 for DCS). Every character in the Dingbats block encodes as E2 9C xx,
// so claude's window title, "✳️ Claude Code", ends at the ✳ and the rest is
// printed onto the grid as text. That is the "suggested prompt appearing in
// the pane" the operator reported. The fix is upstream's to make properly
// and no release carries it, so termwrap - the package that owns the
// emulator (invariant 4) - rewrites the byte before the parser sees it.
//
// What it does not do, and why:
//   - Ground bytes are never touched. Parser.Advance routes UTF-8 sequences in
//     ground through advanceUtf8, which never consults the table, so a ✳ drawn
//     as text is safe; and the guard has no UTF-8 tracking, so it must not
//     treat a 0x9D or 0x90 in ground as a string introducer either.
//   - SOS, PM and APC (ESC X, ESC ^, ESC _) are not guarded: in x/ansi every
//     byte >= 0x80 breaks out of those, not only 0x9C, and claude emits none of
//     them. A guard that half-protects a state is worse than one that says it
//     does not cover it.
//   - The byte becomes '?', not nothing: omatty wires no title callback, so the
//     payload's text is never shown and only needs to stop being a terminator.
type c1Guard struct {
	src   io.Reader
	state guardState
}

type guardState int

const (
	guardGround guardState = iota
	guardEscape            // an ESC was the last byte
	guardString            // inside an OSC or DCS payload
)

// Control bytes the guard reacts to.
const (
	byteESC = 0x1b
	byteBEL = 0x07
	byteCAN = 0x18
	byteSUB = 0x1a
	byteST  = 0x9c
)

// Read fills p from the source and rewrites it in place. The state carries
// across calls: a title can straddle two of the emulator's 4 KiB reads.
func (g *c1Guard) Read(p []byte) (int, error) {
	n, err := g.src.Read(p)
	for i := range p[:n] {
		p[i] = g.scan(p[i])
	}
	return n, err
}

// scan advances the state by one byte and returns the byte to emit.
func (g *c1Guard) scan(b byte) byte {
	switch g.state {
	case guardEscape:
		g.state = afterEscape(b)
	case guardString:
		return g.inPayload(b)
	default:
		if b == byteESC {
			g.state = guardEscape
		}
	}
	return b
}

// afterEscape is the state after ESC and one more byte: ] opens an OSC, P a
// DCS; anything else is some other sequence, back in ground.
func afterEscape(b byte) guardState {
	switch b {
	case ']', 'P':
		return guardString
	case byteESC:
		return guardEscape
	}
	return guardGround
}

// inPayload handles a byte inside a string: BEL, CAN and SUB end it, ESC
// starts the two-byte ST (or aborts it, either way the payload is over), and
// the one byte the parser would misread is replaced.
func (g *c1Guard) inPayload(b byte) byte {
	switch b {
	case byteBEL, byteCAN, byteSUB:
		g.state = guardGround
	case byteESC:
		g.state = guardEscape
	case byteST:
		return '?'
	}
	return b
}
