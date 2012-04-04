package dcpu

import (
	"fmt"
)

type Word uint16

type OpcodeError struct {
	Opcode Word
}

func (err *OpcodeError) Error() string {
	return fmt.Sprintf("invalid opcode %#04x", err.Opcode)
}

type State struct {
	Registers
	Ram Memory
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

const (
	assignableTypeNone = iota
	assignableTypeRegister
	assignableTypeMemory
)

type assignable struct {
	valueType int
	index     Word
}

func (s *State) readAssignable(assignable assignable) (result Word, ok bool) {
	switch assignable.valueType {
	case assignableTypeRegister:
		result = s.Registers[assignable.index]
		ok = true
	case assignableTypeMemory:
		result = s.Ram.GetWord(assignable.index)
		ok = true
	}
	return
}

// When writing to a non-assignable location, returns false, nil
// When writing to a protected location, returns false, error
// Otherwise returns true, nil
func (s *State) writeAssignable(assignable assignable, value Word) (bool, error) {
	switch assignable.valueType {
	case assignableTypeRegister:
		s.Registers[assignable.index] = value
		return true, nil
	case assignableTypeMemory:
		if err := s.Ram.SetWord(assignable.index, value); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func (s *State) nextWord() Word {
	word := s.Ram.GetWord(s.PC())
	s.IncrPC()
	return word
}

func (s *State) skipInstruction() {
	s.SetPC(s.PC() + wordCount(s.Ram.GetWord(s.PC())))
}

func (s *State) translateOperand(op Word) (val Word, assign assignable) {
	switch op {
	case 0, 1, 2, 3, 4, 5, 6, 7:
		// 0-7: register value - register values
		assign = assignable{assignableTypeRegister, op}
	case 8, 9, 10, 11, 12, 13, 14, 15:
		// 8-15: [register value] - value at address in registries
		assign = assignable{assignableTypeMemory, s.Registers[op-8]}
	case 16, 17, 18, 19, 20, 21, 22, 23:
		// 16-23: [next word of ram + register value] - memory address offset by register value
		assign = assignable{assignableTypeMemory, s.nextWord() + s.Registers[op-16]}
	case 24:
		// 24: POP - value at stack address, then increases stack counter
		assign = assignable{assignableTypeMemory, s.SP()}
		s.IncrSP()
	case 25:
		// 25: PEEK - value at stack address
		assign = assignable{assignableTypeMemory, s.SP()}
	case 26:
		// 26: PUSH - decreases stack address, then value at stack address
		s.DecrSP()
		assign = assignable{assignableTypeMemory, s.SP()}
	case 27:
		// 27: SP - current stack pointer value - current stack address
		assign = assignable{assignableTypeRegister, registerSP}
	case 28:
		// 28: PC - program counter- current program counter
		assign = assignable{assignableTypeRegister, registerPC}
	case 29:
		// 29: O - overflow - current value of the overflow
		assign = assignable{assignableTypeRegister, registerO}
	case 30:
		// 30: [next word of ram] - memory address
		assign = assignable{assignableTypeMemory, s.nextWord()}
	case 31:
		// 31: next word of ram - literal, does nothing on assign
		val = s.nextWord()
	default:
		if op >= 64 {
			panic("Out of bounds operand")
		}
		val = op - 32
	}
	if assVal, ok := s.readAssignable(assign); ok {
		val = assVal
	}
	return
}

// Step iterates the CPU by one instruction.
func (s *State) Step() error {
	// fetch
	opcode := s.nextWord()

	// decode
	ins, a, b := decodeOpcode(opcode)

	var assign assignable
	if ins != 0 { // don't translate for the non-basic opcodes
		a, assign = s.translateOperand(a)
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
			_, assign = s.translateOperand(0x1a) // PUSH
			a, _ = s.translateOperand(a)
			s.writeAssignable(assign, s.PC())
			s.SetPC(a)
			assign = assignable{}
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
		s.SetO(Word(result >> 16)) // will always be 0x0 or 0x1
	case 3:
		// SUB a, b - sets a to a-b, sets O to 0xffff if there's an underflow, 0x0 otherwise
		result := uint32(a) - uint32(b)
		val = Word(result & 0xFFFF)
		s.SetO(Word(result >> 16)) // will always be 0x0 or 0xffff
	case 4:
		// MUL a, b - sets a to a*b, sets O to ((a*b)>>16)&0xffff
		result := uint32(a) * uint32(b)
		val = Word(result & 0xFFFF)
		s.SetO(Word(result >> 16))
	case 5:
		// DIV a, b - sets a to a/b, sets O to ((a<<16)/b)&0xffff. if b==0, sets a and O to 0 instead.
		if b == 0 {
			val = 0
			s.SetO(0)
		} else {
			val = a / b
			s.SetO(Word(((uint32(a) << 16) / uint32(b))))
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
		s.SetO(Word(result >> 16))
	case 8:
		// SHR a, b - sets a to a>>b, sets O to ((a<<16)>>b)&0xffff
		val = a >> b
		s.SetO(Word((uint32(a) << 16) >> b))
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
			s.skipInstruction()
		}
		assign = assignable{}
	case 13:
		// IFN a, b - performs next instruction only if a!=b
		if a == b {
			s.skipInstruction()
		}
		assign = assignable{}
	case 14:
		// IFG a, b - performs next instruction only if a>b
		if a <= b {
			s.skipInstruction()
		}
		assign = assignable{}
	case 15:
		// IFB a, b - performs next instruction only if (a&b)!=0
		if (a & b) == 0 {
			s.skipInstruction()
		}
		assign = assignable{}
	default:
		panic("Out of bounds opcode")
	}

	// store
	if _, err := s.writeAssignable(assign, val); err != nil {
		return err
	}

	return nil
}
