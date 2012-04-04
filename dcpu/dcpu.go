package dcpu

import (
	"fmt"
	"unsafe"
)

type Word uint16

type ProtectionError struct {
	Address            Word
	Opcode             Word
	OperandA, OperandB Word
}

func (err *ProtectionError) Error() string {
	return fmt.Sprintf("protection violation at address %#x (instruction %#04x, operands %#x, %#x)",
		err.Address, err.Opcode, err.OperandA, err.OperandB)
}

type OpcodeError struct {
	Opcode Word
}

func (err *OpcodeError) Error() string {
	return fmt.Sprintf("invalid opcode %#04x", err.Opcode)
}

type Registers struct {
	A, B, C, X, Y, Z, I, J Word
	PC                     Word
	SP                     Word
	O                      Word
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

type State struct {
	Registers
	Ram       [0x10000]Word
	Protected []Region
}

func decodeOpcode(opcode Word) (oooo, aaaaaa, bbbbbb Word) {
	oooo = opcode & 0xF
	aaaaaa = (opcode >> 4) & 0x3F
	bbbbbb = (opcode >> 10) & 0x3F
	return
}

// wordCount counts the number of words in the instruction identified by the given opcode
func wordCount(opcode Word) Word {
	_, a, b := decodeOpcode(opcode)
	count := Word(1)
	switch {
	case a >= 16 && a <= 23:
	case a == 30:
	case a == 31:
		count++
	}
	switch {
	case b >= 16 && b <= 23:
	case b == 30:
	case b == 31:
		count++
	}
	return count
}

func (s *State) translateOperand(op Word) (val Word, assignable *Word) {
	switch op {
	// 0-7: register value - register values
	case 0:
		assignable = &s.A
	case 1:
		assignable = &s.B
	case 2:
		assignable = &s.C
	case 3:
		assignable = &s.X
	case 4:
		assignable = &s.Y
	case 5:
		assignable = &s.Z
	case 6:
		assignable = &s.I
	case 7:
		assignable = &s.J
	// 8-15: [register value] - value at address in registries
	case 8:
		assignable = &s.Ram[s.A]
	case 9:
		assignable = &s.Ram[s.B]
	case 10:
		assignable = &s.Ram[s.C]
	case 11:
		assignable = &s.Ram[s.X]
	case 12:
		assignable = &s.Ram[s.Y]
	case 13:
		assignable = &s.Ram[s.Z]
	case 14:
		assignable = &s.Ram[s.I]
	case 15:
		assignable = &s.Ram[s.J]
	// 16-23: [next word of ram + register value] - memory address offset by register value
	case 16:
		assignable = &s.Ram[s.Ram[s.PC]+s.A]
		s.PC++
	case 17:
		assignable = &s.Ram[s.Ram[s.PC]+s.B]
		s.PC++
	case 18:
		assignable = &s.Ram[s.Ram[s.PC]+s.C]
		s.PC++
	case 19:
		assignable = &s.Ram[s.Ram[s.PC]+s.X]
		s.PC++
	case 20:
		assignable = &s.Ram[s.Ram[s.PC]+s.Y]
		s.PC++
	case 21:
		assignable = &s.Ram[s.Ram[s.PC]+s.Z]
		s.PC++
	case 22:
		assignable = &s.Ram[s.Ram[s.PC]+s.I]
		s.PC++
	case 23:
		assignable = &s.Ram[s.Ram[s.PC]+s.J]
		s.PC++
	// 24: POP - value at stack address, then increases stack counter
	case 24:
		assignable = &s.Ram[s.SP]
		s.SP++
	// 25: PEEK - value at stack address
	case 25:
		assignable = &s.Ram[s.SP]
	// 26: PUSH - decreases stack address, then value at stack address
	case 26:
		s.SP--
		assignable = &s.Ram[s.SP]
	// 27: SP - current stack pointer value - current stack address
	case 27:
		assignable = &s.SP
	// 28: PC - program counter- current program counter
	case 28:
		assignable = &s.PC
	// 29: O - overflow - current value of the overflow
	case 29:
		assignable = &s.O
	// 30: [next word of ram] - memory address
	case 30:
		assignable = &s.Ram[s.Ram[s.PC]]
		s.PC++
	// 31: next word of ram - literal, does nothing on assign
	case 31:
		val = s.Ram[s.PC]
		s.PC++
	default:
		if op >= 64 {
			panic("Out of bounds operand")
		}
		val = op - 32
	}
	if assignable != nil {
		val = *assignable
	}
	return
}

func (s *State) isProtected(address Word) bool {
	for _, region := range s.Protected {
		if region.Contains(address) {
			return true
		}
		if region.Start > address {
			break
		}
	}
	return false
}

func (s *State) isProtectedPtr(address *Word) (bool, Word) {
	// are we in our ram?
	ptr := uintptr(unsafe.Pointer(address))
	ramStart := uintptr(unsafe.Pointer(&s.Ram[0]))
	ramEnd := uintptr(unsafe.Pointer(&s.Ram[len(s.Ram)-1]))
	if ptr >= ramStart && ptr <= ramEnd {
		index := Word((ptr - ramStart) / unsafe.Sizeof(s.Ram[0]))
		return s.isProtected(index), index
	}
	return false, 0
}

// Step iterates the CPU by one instruction.
func (s *State) Step() error {
	// fetch
	opcode := s.Ram[s.PC]
	s.PC++

	// decode
	ins, a, b := decodeOpcode(opcode)

	var assignable *Word
	if ins != 0 { // don't translate for the non-basic opcodes
		a, assignable = s.translateOperand(a)
		b, _ = s.translateOperand(b)
	}

	// execute
	var val Word
	switch ins {
	case 0:
		// non-basic opcodes
		ins, a = a, b
		switch ins {
		case 1:
			// JSR a - pushes the address of the next instruction to the stack, then sets PC to a
			_, assignable = s.translateOperand(0x1a) // PUSH
			a, _ = s.translateOperand(a)
			if ok, index := s.isProtectedPtr(assignable); ok {
				return &ProtectionError{
					Address:  index,
					Opcode:   opcode,
					OperandA: a,
				}
			}
			val = s.PC
			s.PC = a
		default:
			return &OpcodeError{opcode}
		}
	case 1:
		// SET a, b - sets a to b
		val = b
	case 2:
		// ADD a, b - sets a to a+b, sets O to 0x0001 if there's an overflow, 0x0 otherwise
		result := uint32(a) + uint32(b)
		val = Word(result & 0xFFFF)
		s.O = Word(result >> 16) // will always be 0x0 or 0x1
	case 3:
		// SUB a, b - sets a to a-b, sets O to 0xffff if there's an underflow, 0x0 otherwise
		result := uint32(a) - uint32(b)
		val = Word(result & 0xFFFF)
		s.O = Word(result >> 16) // will always be 0x0 or 0xffff
	case 4:
		// MUL a, b - sets a to a*b, sets O to ((a*b)>>16)&0xffff
		result := uint32(a) * uint32(b)
		val = Word(result & 0xFFFF)
		s.O = Word(result >> 16)
	case 5:
		// DIV a, b - sets a to a/b, sets O to ((a<<16)/b)&0xffff. if b==0, sets a and O to 0 instead.
		if b == 0 {
			val, s.O = 0, 0
		} else {
			val = a / b
			s.O = Word(((uint32(a) << 16) / uint32(b)))
		}
	case 6:
		// MOD a, b - sets a to a%b. if b==0, sets a to 0 instead.
		if b == 0 {
			val = 0
		} else {
			val = a % b
		}
	case 7:
		// SHL a, b - sets a to a<<b, sets O to ((a<<b)>>16)&0xffff
		result := uint32(a) << uint32(b)
		val = Word(result & 0xFFFF)
		s.O = Word(result >> 16)
	case 8:
		// SHR a, b - sets a to a>>b, sets O to ((a<<16)>>b)&0xffff
		val = a >> b
		s.O = Word((uint32(a) << 16) >> b)
	case 9:
		// AND a, b - sets a to a&b
		val = a & b
	case 10:
		// BOR a, b - sets a to a|b
		val = a | b
	case 11:
		// XOR a, b - sets a to a^b
		val = a ^ b
	case 12:
		// IFE a, b - performs next instruction only if a==b
		if a != b {
			s.PC += wordCount(s.Ram[s.PC])
		}
	case 13:
		// IFN a, b - performs next instruction only if a!=b
		if a == b {
			s.PC += wordCount(s.Ram[s.PC])
		}
	case 14:
		// IFG a, b - performs next instruction only if a>b
		if a <= b {
			s.PC += wordCount(s.Ram[s.PC])
		}
	case 15:
		// IFB a, b - performs next instruction only if (a&b)!=0
		if (a & b) == 0 {
			s.PC += wordCount(s.Ram[s.PC])
		}
	default:
		panic("Out of bounds opcode")
	}

	// store
	if ins >= 1 && ins <= 11 && assignable != nil {
		// test memory protection
		if ok, index := s.isProtectedPtr(assignable); ok {
			// protection error
			return &ProtectionError{
				Address:  index,
				Opcode:   opcode,
				OperandA: a,
				OperandB: b,
			}
		}
		// go ahead and store
		*assignable = val
	}

	return nil
}
