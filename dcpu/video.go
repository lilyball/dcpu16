package dcpu

import (
	"errors"
	"github.com/kballard/dcpu16/dcpu/core"
	"github.com/kballard/termbox-go"
)

// For the moment, assume an 80 column terminal
const (
	windowWidth = 80
)

type Video struct {
	words     [0x400]core.Word
	addresses <-chan core.Word
}

func (v *Video) Init() error {
	return termbox.Init()
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
	row := int(offset / windowWidth)
	column := int(offset % windowWidth)

	termWidth, termHeight := termbox.Size()
	if row >= termHeight || column >= termWidth {
		// this is offscreen, so do nothing
		return
	}
	ch := rune(v.words[offset] & 0x7F)
	// color seems to be in the top 2 nibbles, MSB being FG and LSB are BG
	// Within each nibble, from LSB to MSB, is blue, green, red, highlight
	// Lastly, the bit at 0x80 is apparently blink.
	flag := (v.words[offset] & 0x80) != 0
	colors := byte((v.words[offset] & 0xFF00) >> 8)
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
	return
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
