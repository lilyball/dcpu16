package dcpu

import (
	"./core"
	"errors"
	"github.com/nsf/termbox-go"
)

// For the moment, assume an 80 column terminal
const (
	windowWidth = 80
)

type Video struct {
	words    [0x400]core.Word
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
		return
	}
	ch := rune(v.words[offset] & 0x7F)
	fg, bg := termbox.ColorDefault, termbox.ColorDefault // figure out colors later
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
	addresses := make(chan core.Word)
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
