package core

import (
	"errors"
	"fmt"
	"io"
	"sort"
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
	mapped    []MMIORegion
}

func (m *Memory) Load(offset Word) Word {
	for _, region := range m.mapped {
		if region.Contains(offset) {
			return region.get(offset - region.Start)
		}
	}
	return m.ram[offset]
}

func (m *Memory) Store(offset, value Word) error {
	for _, region := range m.mapped {
		if region.Contains(offset) {
			return region.set(offset-region.Start, value)
		}
	}
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

type MMIORegion struct {
	Region
	get func(address Word) Word
	set func(address, val Word) error
}

// MapRegion maps a region of memory to a pair of get/set functions.
// If set returns an error, the machine is halted.
// The address in both functions is relative to the start of the region.
func (m *Memory) MapRegion(start, length Word, get func(address Word) Word, set func(address, val Word) error) error {
	if int(start)+int(length) > len(m.ram) {
		return ErrOutOfBounds
	}
	for _, region := range m.mapped {
		if region.Contains(start) || region.Contains(length) {
			return errors.New("MapRegion: this region conflicts with an existing mapped region")
		} else if region.Start > start+length {
			break
		}
	}
	m.mapped = append(m.mapped, MMIORegion{
		Region: Region{start, length},
		get:    get,
		set:    set,
	})
	return nil
}

// UnampRegion only unmaps if the region precisely matches an existing mapped region
func (m *Memory) UnmapRegion(start, length Word) error {
	if int(start)+int(length) > len(m.ram) {
		return ErrOutOfBounds
	}
	for i, region := range m.mapped {
		if region.Start == start && region.Length == length {
			// this is the one
			copy(m.mapped[i:], m.mapped[i+1:])
			return nil
		} else if region.Start > start {
			break
		}
	}
	return errors.New("UnmapRegion: no region matches the input")
}

// Writes all non-zero rows of memory to the writer in the format
// 0000: 1111 2222 3333 4444 5555 6666 7777 8888
// highlights is a slice of addresses that should be highlighted
// when emitted. Primarily intended for highlighting PC. Note that
// an otherwise-zero row will still be emitted if a word needs to
// be highlighted.
func (m *Memory) DumpMemory(w io.Writer, highlights []int) error {
	var hslice []int
	hnext := -1
	if len(highlights) > 0 {
		// copy and sort the highlights
		hslice = make([]int, len(highlights))
		copy(hslice, highlights)
		sort.Ints(hslice)
		hnext = hslice[0]
		hslice = hslice[1:]
	}
	const width = 8
	for i, j := 0, width; j < len(m.ram); i, j = i+width, j+width {
		var nonzero bool
		if hnext >= i && hnext < j {
			nonzero = true
		} else {
			// test the memory for a non-zero
			for k := i; k < j; k++ {
				if m.ram[k] != 0 {
					nonzero = true
					break
				}
			}
		}
		if nonzero {
			// print the row
			if _, err := io.WriteString(w, fmt.Sprintf("%04x:", i)); err != nil {
				return err
			}
			for k := i; k < j; k++ {
				start, end := "", ""
				if hnext == k {
					start = "\033[44m"
					end = "\033[m"
					if len(hslice) > 0 {
						hnext = hslice[0]
						hslice = hslice[1:]
					}
				}
				if _, err := io.WriteString(w, fmt.Sprintf(" %s%04x%s", start, m.ram[k], end)); err != nil {
					return err
				}
			}
			if _, err := w.Write([]byte{'\n'}); err != nil {
				return err
			}
		}
	}
	return nil
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
