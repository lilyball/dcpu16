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
	words     [0x400]core.Word
	addresses <-chan core.Word
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

func (v *Video) HandleChanges() {
	var offset core.Word
	select {
	case offset = <-v.addresses:
	default:
		return
	}
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
	var color termbox.Attribute = termbox.Attribute(v.words[backgroundColorAddress] & 0x7) + termbox.ColorBlack

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

func (v *Video) MapToMachine(offset core.Word, m *Machine) error {
	if v.addresses != nil {
		return errors.New("Video is already mapped to a machine")
	}
	addresses := make(chan core.Word, 1)
	get := func(offset core.Word) core.Word {
		return v.words[offset]
	}
	set := func(offset, val core.Word) error {
		v.words[offset] = val
		addresses <- offset
		return nil
	}
	v.addresses = addresses
	return m.State.Ram.MapRegion(offset, core.Word(len(v.words)), get, set)
}

func (v *Video) UnmapFromMachine(offset core.Word, m *Machine) error {
	if v.addresses == nil {
		return errors.New("Video is not mapped to a machine")
	}
	if err := m.State.Ram.UnmapRegion(offset, core.Word(len(v.words))); err != nil {
		return err
	}
	v.addresses = nil
	return nil
}
