package dcpu

import (
	"errors"
	"github.com/kballard/dcpu16/dcpu/core"
	"github.com/kballard/termbox-go"
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

type Video struct {
	words  [0x400]core.Word
	mapped bool
}

func (v *Video) Init() error {
	if err := termbox.Init(); err != nil {
		return err
	}
	// Default the background to cyan, for the heck of it
	v.words[0x0280] = 6

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
	// Lastly, the bit at 0x80 is apparently blink.
	flag := (word & 0x80) != 0
	colors := byte((word & 0xFF00) >> 8)
	fgNibble := (colors & 0xF0) >> 4
	bgNibble := colors & 0x0F
	colorToAttr := func(color byte) termbox.Attribute {
		attr := termbox.ColorDefault
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
	fg, bg := colorToAttr(fgNibble), colorToAttr(bgNibble)
	if flag {
		fg |= termbox.AttrBlink
	}
	termbox.SetCell(column, row, ch, fg, bg)
}

func (v *Video) drawBorder() {
	// we have no good information on the background color lookup at the moment
	// So instead just treat the low 3 bits as an ANSI color
	// Take advantage of the fact that termbox colors are in the same order as ANSI colors
	var color termbox.Attribute = termbox.Attribute(v.words[backgroundColorAddress]&0x7) + termbox.ColorBlack

	// draw top/bottom
	for _, row := range [2]int{0, windowHeight + 1} {
		for col := 0; col < windowWidth+2; col++ {
			termbox.SetCell(col, row, ' ', termbox.ColorDefault, color)
		}
	}
	// draw left/right
	for _, col := range [2]int{0, windowWidth + 1} {
		for row := 1; row < windowHeight+1; row++ {
			termbox.SetCell(col, row, ' ', termbox.ColorDefault, color)
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
