// DCPU-16 keyboard implementation
// The keyboard is a 16-word circular buffer at 0x9000
// After a key is read, the program needs to stuff 0 back into the spot.
// It's not fully-documented, but my assumption is if the circular buffer
// runs out of space, subsequent keys are dropped.

package dcpu

import (
	"errors"
	"github.com/kballard/dcpu16/dcpu/core"
)

type Keyboard struct {
	words  [0x10]core.Word
	input  chan rune
	offset int
}

// PollKeys checks for any pending keys and stuffs them into the buffer
func (k *Keyboard) PollKeys() {
	if k.words[k.offset] == 0 {
		// we have an open spot; check for a key
		select {
		case key := <-k.input:
			k.words[k.offset] = core.Word(key)
			k.offset = (k.offset + 1) % len(k.words)
		default:
		}
	}
}

func (k *Keyboard) MapToMachine(offset core.Word, m *Machine) error {
	if k.input != nil {
		return errors.New("Keyboard is already mapped to a machine")
	}
	k.input = make(chan rune, 1)
	k.offset = 0
	for i := 0; i < 10; i++ {
		// zero out the words
		k.words[i] = 0
	}
	get := func(offset core.Word) core.Word {
		return k.words[offset]
	}
	set := func(offset, val core.Word) error {
		k.words[offset] = val
		return nil
	}
	return m.State.Ram.MapRegion(offset, core.Word(len(k.words)), get, set)
}

func (k *Keyboard) UnmapFromMachine(offset core.Word, m *Machine) error {
	if k.input == nil {
		return errors.New("Keyboard is not mapped to a machine")
	}
	if err := m.State.Ram.UnmapRegion(offset, core.Word(len(k.words))); err != nil {
		return err
	}
	close(k.input)
	k.input = nil
	return nil
}

var remapMap = map[rune]rune{
	'\x7F': '\x08', // fix delete on OS X
	'\x0D': '\x0A', // fix return on OS X
	0xffed: 128,    // arrow up
	0xffec: 129,    // arrow down
	0xffeb: 130,    // arrow left
	0xffea: 131,    // arrow right
}

var keyUpMap = map[rune]bool{
	128: true,
	129: true,
	130: true,
	131: true,
}

func (k *Keyboard) RegisterKey(key rune) {
	// process any remappings first
	if k2, ok := remapMap[key]; ok {
		key = k2
	}
	select {
	case k.input <- key:
		if keyUpMap[key] {
			// if we sent the keydown, we must send the keyup unconditionally
			k.input <- key | 0x100
		}
	default:
	}
}
