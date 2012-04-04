package dcpu

import (
	"errors"
)

var ErrOutOfBounds = errors.New("out of bounds")

// LoadProgram loads a program from the given slice into Ram at the given offset.
// Returns ErrOutOfBounds if the program exceeds the bounds of Ram.
func (s *State) LoadProgram(input []Word, offset Word) error {
	if len(input)+int(offset) > len(s.Ram) {
		return ErrOutOfBounds
	}
	copy(s.Ram[offset:], input)
	return nil
}
