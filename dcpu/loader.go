package dcpu

import (
	"errors"
)

var ErrOutOfBounds = errors.New("out of bounds")

// LoadProgram loads a program from the given slice into Ram at the given offset.
// Returns ErrOutOfBounds if the program exceeds the bounds of Ram.
// Optionally protects the region.
func (s *State) LoadProgram(input []Word, offset Word, protected bool) error {
	if len(input)+int(offset) > len(s.Ram) {
		return ErrOutOfBounds
	}
	copy(s.Ram[offset:], input)
	return s.MemProtect(offset, Word(len(input)), protected)
}

// MemProtect marks a region of memory as protected (or unprotected).
// Returns ErrOutOfBounds if the region exceeds the bounds of Ram.
func (s *State) MemProtect(offset, length Word, protected bool) error {
	if int(offset)+int(length) > len(s.Ram) {
		return ErrOutOfBounds
	}
	if protected {
		if s.Protected == nil {
			s.Protected = []Region{{offset, length}}
		} else {
			// try to unify with any existing regions
			// we'd use a range expression but we might have to delete entries
			for i := 0; i < len(s.Protected); i++ {
				region := &s.Protected[i]
				if region.Start > offset+length {
					// we've found our insertion point
					s.Protected = append(s.Protected, Region{})
					copy(s.Protected[i+1:], s.Protected[i:])
					s.Protected[i] = Region{offset, length}
					break
				}
				if region.End() > offset {
					// we've found overlap
					if i+1 < len(s.Protected) && s.Protected[i+1].Start < offset+length {
						// we're bridging two protected regions. Unify them
						next := s.Protected[i+1]
						region.Length = next.End() - region.Start
						copy(s.Protected[i+1:], s.Protected[i+2:])
						s.Protected = s.Protected[:len(s.Protected)-1]
					} else {
						// we're extending a region
						*region = region.Union(Region{offset, length})
					}
					break
				}
			}
		}
	} else if s.Protected != nil {
		// we'd use a range expression but we might end up deleting the current entry
		for i := 0; i < len(s.Protected); i++ {
			region := &s.Protected[i]
			if region.Start > offset+length {
				break
			}
			if region.End() > offset {
				// we've got at least partial overlap
				if region.Start >= offset {
					// region is starting inside the area
					if !region.Contains(offset + length) {
						// total overlap
						copy(s.Protected[i:], s.Protected[i+1:])
						s.Protected = s.Protected[:len(s.Protected)-1]
					} else {
						// region extends past our end
						region.Start, region.Length = offset+length, region.End()-(offset+length)
					}
				} else {
					// region starts before our area and extends into it
					region.Length = offset - region.Start
				}
			}
		}
	}
	return nil
}
