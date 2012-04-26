package main

// remap keys from termbox

import (
	"github.com/kballard/dcpu16/dcpu"
	"github.com/kballard/termbox-go"
)

var keymapTermboxKeyToRune = map[termbox.Key]rune{
	termbox.KeyDelete: 127,
	termbox.KeySpace:  0x20,
}

var keymapTermboxKeyToKey = map[termbox.Key]dcpu.Key{
	termbox.KeyArrowUp:    dcpu.KeyArrowUp,
	termbox.KeyArrowDown:  dcpu.KeyArrowDown,
	termbox.KeyArrowLeft:  dcpu.KeyArrowLeft,
	termbox.KeyArrowRight: dcpu.KeyArrowRight,
}

var keymapRuneToRune = map[rune]rune{
	'\x7F': '\x08', // fix delete on OS X
	'\x0D': '\x0A', // fix return on OS X
}
