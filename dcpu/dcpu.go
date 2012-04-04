package dcpu

type Word uint16

type Registers struct {
	A, B, C, X, Y, Z, I, J Word
	PC                     Word
	SP                     Word
	O                      Word
}

type State struct {
	Registers
	Ram [0x10000]Word
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
		assignable = &s.Ram[s.PC+s.A]
		s.PC++
	case 17:
		assignable = &s.Ram[s.PC+s.B]
		s.PC++
	case 18:
		assignable = &s.Ram[s.PC+s.C]
		s.PC++
	case 19:
		assignable = &s.Ram[s.PC+s.X]
		s.PC++
	case 20:
		assignable = &s.Ram[s.PC+s.Y]
		s.PC++
	case 21:
		assignable = &s.Ram[s.PC+s.Z]
		s.PC++
	case 22:
		assignable = &s.Ram[s.PC+s.I]
		s.PC++
	case 23:
		assignable = &s.Ram[s.PC+s.J]
		s.PC++
	// 24: POP - value at stack address, then increases stack counter
	case 24:
		assignable = &s.Ram[s.SP]
		s.SP++
	// 25: PEEK - value at stack address
	case 25:
		assignable = &s.Ram[s.SP]
	case 26:
		// 26: PUSH - decreases stack address, then value at stack address
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

// Step iterates the CPU by one instruction.
func (s *State) Step() {
	// fetch
	opcode := s.Ram[s.PC]
	s.PC++

	// decode
	ins := opcode & 0xF
	a := (opcode >> 4) & 0x3F
	b := (opcode >> 10) & 0x3F
	var assignable *Word

	a, assignable = s.translateOperand(a)
	b, _ = s.translateOperand(b)

	// execute
	var val Word
	switch ins {
	case 0:
		// marked RESERVED, lets just treat it as a NOP
	case 1:
		// SET a, b - sets value of b to a
		val = b
	case 2:
		// ADD a, b - adds b to a, sets O
		result := uint32(a) + uint32(b)
		val = Word(result & 0xFFFF)
		s.O = Word(result >> 16)
	case 3:
		// SUB a, b - subtracts b from a, sets O
		result := uint32(a) - uint32(b)
		val = Word(result & 0xFFFF)
		s.O = Word(result >> 16)
	case 4:
		// MUL a, b - multiplies a by b, sets O
		result := uint32(a) * uint32(b)
		val = Word(result & 0xFFFF)
		s.O = Word(result >> 16)
	case 5:
		// DIV a, b - divides a by b, sets O
		// NB: how can this overflow?
		// assuming for the moment that O is supposed to be the mod
		val = a / b
		s.O = a % b
	case 6:
		// MOD a, b - remainder of a over b
		val = a % b
	case 7:
		// SHL a, b - shifts a left b places, sets O
		result := uint32(a) << uint32(b)
		val = Word(result & 0xFFFF)
		s.O = Word(result >> 16)
	case 8:
		// SHR a, b - shifts a right b places, sets O
		// NB: how can this overflow?
		val = a >> b
	case 9:
		// AND a, b - binary and of a and b
		val = a & b
	case 10:
		// BOR a, b - binary or of a and b
		val = a | b
	case 11:
		// XOR a, b - binary xor of a and b
		val = a ^ b
	case 12:
		// IFE a, b - skips one instruction if a!=b
		if a != b {
			s.PC++
		}
	case 13:
		// IFN a, b - skips one instruction if a==b
		if a == b {
			s.PC++
		}
	case 14:
		// IFG a, b - skips one instruction if a<=b
		if a <= b {
			s.PC++
		}
	case 15:
		// IFB a, b - skips one instruction if (a&b)==0
		if (a & b) == 0 {
			s.PC++
		}
	default:
		panic("Out of bounds opcode")
	}

	// store
	if ins >= 1 && ins <= 11 && assignable != nil {
		*assignable = val
	}
}
