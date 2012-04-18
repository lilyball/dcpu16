package dcpu

import (
	"errors"
	"github.com/kballard/dcpu16/dcpu/core"
	"github.com/kballard/termbox-go"
	"os"
	"strings"
)

// The display is 32x12 (128x96 pixels) surrounded by a
// 16 pixel border / background.
//
// We can't handle pixels, so use a 32x12 character display, with a border
// of one character.
const (
	windowWidth            = 32
	windowHeight           = 12
	characterRangeStart    = 0x0180
	miscRangeStart         = 0x0280
	backgroundColorAddress = 0x0280
)

const DefaultScreenRefreshRate ClockRate = 60 // 60Hz

var supportsXterm256 bool

// colorToAnsi maps the 4-bit DCPU-16 colors to xterm-256 colors
// We can't do an exat match, but we can get pretty close.
// 0x55 becomes 0x66
// 0xAA becomes 0x99
// 0xFF is left as-is
// Note: color spec says +red, +green, -highlight puts the green channel
// at 0xFF instead of 0xAA. After reading comments on the 0x10cwiki, this
// is likely a bug, it should probably be dropped to 0x55. Also note that
// this only holds if blue is off.
var colorToAnsi [16]byte = [...]byte{
	/* 0000 */ 16 /* 0001 */, 19 /* 0010 */, 34 /* 0011 */, 37,
	/* 0100 */ 124 /* 0101 */, 127 /* 0110 */, 136 /* 0111 */, 145,
	/* 1000 */ 102 /* 1001 */, 105 /* 1010 */, 120 /* 1011 */, 123,
	/* 1100 */ 210 /* 1101 */, 213 /* 1110 */, 228 /* 1111 */, 231,
}

type Video struct {
	RefreshRate ClockRate // the refresh rate of the screen
	words       [0x400]core.Word
	mapped      bool
}

func (v *Video) Init() error {
	if err := termbox.Init(); err != nil {
		return err
	}
	// Default the background to cyan, for the heck of it
	v.words[0x0280] = 3

	v.clearDisplay()
	v.drawBorder()

	return nil
}

func (v *Video) Close() {
	termbox.Close()
}

func (v *Video) handleChange(offset core.Word) {
	if offset < characterRangeStart {
		row := int(offset / windowWidth)
		column := int(offset % windowWidth)
		v.updateCell(row, column, v.words[offset])
	} else if offset < miscRangeStart {
		// we can't handle font stuff with the terminal
	} else if offset == backgroundColorAddress {
		v.drawBorder()
	}
}

func (v *Video) updateCell(row, column int, word core.Word) {
	// account for the border
	row++
	column++

	ch := rune(word & 0x7F)
	if ch == 0 {
		// replace 0000 with space
		ch = 0x20
	}
	// color seems to be in the top 2 nibbles, MSB being FG and LSB are BG
	// Within each nibble, from LSB to MSB, is blue, green, red, highlight
	// Lastly, the bit at 0x80 is blink.
	flag := (word & 0x80) != 0
	colors := byte((word & 0xFF00) >> 8)
	fgNibble := (colors & 0xF0) >> 4
	bgNibble := colors & 0x0F
	fg, bg := colorToAttr(fgNibble), colorToAttr(bgNibble)
	if flag {
		fg |= termbox.AttrBlink
	}
	termbox.SetCell(column, row, ch, fg, bg)
}

func colorToAttr(color byte) termbox.Attribute {
	var attr termbox.Attribute
	if supportsXterm256 {
		// We need to use xterm-256 colors to work properly here.
		// Luckily, we built a table!
		attr = termbox.ColorXterm256
		ansi := colorToAnsi[color]
		attr |= termbox.Attribute(ansi) << termbox.XtermColorShift
	} else {
		// We don't seem to support xterm-256 colors, so fall back on
		// trying to use the normal ANSI colors
		attr = termbox.ColorDefault
		// bold
		if color&0x8 != 0 {
			attr |= termbox.AttrBold
		}
		// cheat a bit here. We know the termbox color attributes go in the
		// same order as the ANSI colors, and they're monotomically-incrementing.
		// Just figure out the ANSI code and add ColorBlack
		ansi := termbox.Attribute(0)
		if color&0x1 != 0 {
			// blue
			ansi |= 0x4
		}
		if color&0x2 != 0 {
			// green
			ansi |= 0x2
		}
		if color&0x4 != 0 {
			// red
			ansi |= 0x1
		}
		attr |= ansi + termbox.ColorBlack
		return attr
	}
	return attr
}

func (v *Video) drawBorder() {
	// we have no good information on the background color lookup at the moment
	// So instead just treat the low 4 bits
	color := byte(v.words[backgroundColorAddress] & 0xf)
	attr := colorToAttr(color)

	// draw top/bottom
	for _, row := range [2]int{0, windowHeight + 1} {
		for col := 0; col < windowWidth+2; col++ {
			termbox.SetCell(col, row, ' ', termbox.ColorDefault, attr)
		}
	}
	// draw left/right
	for _, col := range [2]int{0, windowWidth + 1} {
		for row := 1; row < windowHeight+1; row++ {
			termbox.SetCell(col, row, ' ', termbox.ColorDefault, attr)
		}
	}
}

func (v *Video) clearDisplay() {
	// clear all cells inside of the border
	attr := termbox.ColorBlack

	for row := 1; row <= windowHeight; row++ {
		for col := 1; col <= windowWidth; col++ {
			termbox.SetCell(col, row, ' ', termbox.ColorDefault, attr)
		}
	}
}

func (v *Video) Flush() {
	termbox.Flush()
}

func (v *Video) UpdateStats(state *core.State, cycleCount uint) {
	// draw stats below the display
	// Cycles: ###########  PC: 0x####
	// A: 0x####  B: 0x####  C: 0x####  I: 0x####
	// X: 0x####  Y: 0x####  Z: 0x####  J: 0x####
	// O: 0x#### SP: 0x####

	row := windowHeight + 2 /* border */ + 1 /* spacing */
	fg, bg := termbox.ColorDefault, termbox.ColorDefault
	termbox.DrawStringf(1, row, fg, bg, "Cycles: %-11d  PC: %#04x", cycleCount, state.PC())
	row++
	termbox.DrawStringf(1, row, fg, bg, "A: %#04x  B: %#04X  C: %#04x  I: %#04x", state.A(), state.B(), state.C(), state.I())
	row++
	termbox.DrawStringf(1, row, fg, bg, "X: %#04x  Y: %#04x  Z: %#04x  J: %#04x", state.X(), state.Y(), state.Z(), state.J())
	row++
	termbox.DrawStringf(1, row, fg, bg, "O: %#04x SP: %#04x", state.O(), state.SP())
}

func (v *Video) MapToMachine(offset core.Word, m *Machine) error {
	if v.mapped {
		return errors.New("Video is already mapped to a machine")
	}
	get := func(offset core.Word) core.Word {
		return v.words[offset]
	}
	set := func(offset, val core.Word) error {
		v.words[offset] = val
		v.handleChange(offset)
		return nil
	}
	if err := m.State.Ram.MapRegion(offset, core.Word(len(v.words)), get, set); err != nil {
		return err
	}
	v.mapped = true
	return nil
}

func (v *Video) UnmapFromMachine(offset core.Word, m *Machine) error {
	if !v.mapped {
		return errors.New("Video is not mapped to a machine")
	}
	if err := m.State.Ram.UnmapRegion(offset, core.Word(len(v.words))); err != nil {
		return err
	}
	v.mapped = false
	return nil
}

// test for xterm-256 color support
func init() {
	// Check $TERM for the -256color suffix
	supportsXterm256 = strings.HasSuffix(os.ExpandEnv("$TERM"), "-256color")
}
