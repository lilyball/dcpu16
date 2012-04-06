package core

import (
	"errors"
	"fmt"
)

type ProtectionError struct {
	Address Word
}

func (err *ProtectionError) Error() string {
	return fmt.Sprintf("protection violation at address %#x", err.Address)
}

var ErrOutOfBounds = errors.New("out of bounds")

type Memory struct {
	ram       [0x10000]Word
	protected []Region
}

func (m *Memory) GetWord(offset Word) Word {
	return m.ram[offset]
}

func (m *Memory) SetWord(offset, value Word) error {
	for _, region := range m.protected {
		if region.Contains(offset) {
			return &ProtectionError{offset}
		} else if region.Start > offset {
			break
		}
	}
	m.ram[offset] = value
	return nil
}

// GetSlice is intended for testing purposes
func (m Memory) GetSlice(start, end Word) []Word {
	return m.ram[start:end]
}

type Region struct {
	Start  Word
	Length Word
}

func (r Region) Contains(address Word) bool {
	return address >= r.Start && address < r.Start+r.Length
}

// End() returns the first address not contained in the region
func (r Region) End() Word {
	return r.Start + r.Length
}

func (r Region) Union(r2 Region) Region {
	var reg Region
	if r2.Start < r.Start {
		reg.Start = r2.Start
	} else {
		reg.Start = r.Start
	}
	if r2.End() > r.End() {
		reg.Length = r2.End() - reg.Start
	} else {
		reg.Length = r.End() - reg.Start
	}
	return reg
}

// LoadProgram loads a program from the given slice into Ram at the given offset.
// Returns ErrOutOfBounds if the program exceeds the bounds of Ram.
func (s *State) LoadProgram(input []Word, offset Word) error {
	if len(input)+int(offset) > len(s.Ram.ram) {
		return ErrOutOfBounds
	}
	copy(s.Ram.ram[offset:], input)
	return nil
}

// MemProtect marks a region of memory as protected (or unprotected).
// Returns ErrOutOfBounds if the region exceeds the bounds of Ram.
func (s *State) MemProtect(offset, length Word, protected bool) error {
	if int(offset)+int(length) > len(s.Ram.ram) {
		return ErrOutOfBounds
	}
	if protected {
		if s.Ram.protected == nil {
			s.Ram.protected = []Region{{offset, length}}
		} else {
			// try to unify with any existing regions
			// we'd use a range expression but we might have to delete entries
			for i := 0; i < len(s.Ram.protected); i++ {
				region := &s.Ram.protected[i]
				if region.Start > offset+length {
					// we've found our insertion point
					s.Ram.protected = append(s.Ram.protected, Region{})
					copy(s.Ram.protected[i+1:], s.Ram.protected[i:])
					s.Ram.protected[i] = Region{offset, length}
					break
				}
				if region.End() > offset {
					// we've found overlap
					if i+1 < len(s.Ram.protected) && s.Ram.protected[i+1].Start < offset+length {
						// we're bridging two protected regions. Unify them
						next := s.Ram.protected[i+1]
						region.Length = next.End() - region.Start
						copy(s.Ram.protected[i+1:], s.Ram.protected[i+2:])
						s.Ram.protected = s.Ram.protected[:len(s.Ram.protected)-1]
					} else {
						// we're extending a region
						*region = region.Union(Region{offset, length})
					}
					break
				}
			}
		}
	} else if s.Ram.protected != nil {
		// we'd use a range expression but we might end up deleting the current entry
		for i := 0; i < len(s.Ram.protected); i++ {
			region := &s.Ram.protected[i]
			if region.Start > offset+length {
				break
			}
			if region.End() > offset {
				// we've got at least partial overlap
				if region.Start >= offset {
					// region is starting inside the area
					if !region.Contains(offset + length) {
						// total overlap
						copy(s.Ram.protected[i:], s.Ram.protected[i+1:])
						s.Ram.protected = s.Ram.protected[:len(s.Ram.protected)-1]
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
