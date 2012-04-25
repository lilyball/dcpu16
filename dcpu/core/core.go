package core

import (
	"fmt"
)

type Word uint16

type OpcodeError struct {
	Opcode byte
}

func (err *OpcodeError) Error() string {
	return fmt.Sprintf("invalid opcode %#04x", err.Opcode)
}

type State struct {
	Registers
	Ram       Memory
	lastError error   // once set, will be returned always
	step      int     // fetch, decode, execute
	cycleCost uint    // remaining cost of the opcode to execute
	op, a, b  uint32  // operands and opcode (uint32 datatype used for math)
	delayed   bool    // indicates whether we've already delayed the operand fetch
	address   Address // location to store the result
}

const (
	stateStepFetch   = iota // fetch next instruction
	stateStepDecodeA        // process the A operand
	stateStepDecodeB        // process the B operand
	stateStepExecute        // execute the instruction
)

type Address struct {
	addressType int
	index       Word
}

const (
	addressTypeNone = iota
	addressTypeRegister
	addressTypeMemory
)

// StepCycle steps one cycle and returns.
// If the machine halts, the relevant error is returned.
// If the machine was already halted, the same error will be
// returned from all future calls.
func (s *State) StepCycle() error {
	if s.lastError != nil {
		return s.lastError
	}

step:
	switch s.step {
	case stateStepFetch:
		// Fetch the next opcode
		opcode := s.nextWord()
		s.op, s.a, s.b = decodeOpcode(opcode)
		if cost, err := cycleCost(s.op); err != nil {
			s.lastError = err
			return err
		} else {
			s.cycleCost = cost
		}
		s.address = Address{}
		s.delayed = false
		s.step = stateStepDecodeA
		fallthrough
	case stateStepDecodeA:
		// decode operand A
		val, loc, delay := s.fetchOperand(s.a, s.delayed, false)
		s.delayed = delay
		if delay {
			break
		}
		s.a = uint32(val)
		s.address = loc
		if s.op >= opcodeExtendedOffset {
			s.step = stateStepExecute
		} else {
			s.step = stateStepDecodeB
		}
		fallthrough
	case stateStepDecodeB:
		// decode operand B
		val, _, delay := s.fetchOperand(s.b, s.delayed, true)
		s.delayed = delay
		if delay {
			break
		}
		s.b = uint32(val)
		s.step = stateStepExecute
		fallthrough
	case stateStepExecute:
		// execute the instruction
		if s.cycleCost > 1 {
			s.cycleCost--
			break
		}
		// we now have valid opcodes, and we've spun enough cycles for the instruction
		var val Word
		switch s.op {
		case opcodeSET:
			val = Word(s.b)
		case opcodeADD:
			result := s.a + s.b
			val = Word(result)
			s.SetEX(Word(result >> 16))
		case opcodeSUB:
			result := s.a - s.b
			val = Word(result)
			s.SetEX(Word(result >> 16))
		case opcodeMUL:
			result := s.a * s.b
			val = Word(result)
			s.SetEX(Word(result >> 16))
		case opcodeDIV:
			if s.b == 0 {
				val = 0
				s.SetEX(0)
			} else {
				result := s.a / s.b
				val = Word(result)
				// EX is a bit weird here
				s.SetEX(Word((s.a << 16) / s.b))
			}
		case opcodeMOD:
			if s.b == 0 {
				val = 0
			} else {
				val = Word(s.a % s.b)
			}
		case opcodeSHL:
			result := s.a << s.b
			val = Word(result)
			s.SetEX(Word(result >> 16))
		case opcodeSHR:
			val = Word(s.a >> s.b)
			s.SetEX(Word((s.a << 16) >> s.b))
		case opcodeAND:
			val = Word(s.a & s.b)
		case opcodeBOR:
			val = Word(s.a | s.b)
		case opcodeXOR:
			val = Word(s.a ^ s.b)
		case opcodeIFE:
			if !(s.a == s.b) {
				s.skipInstruction()
				break step
			}
			s.address = Address{}
		case opcodeIFN:
			if !(s.a != s.b) {
				s.skipInstruction()
				break step
			}
			s.address = Address{}
		case opcodeIFG:
			if !(s.a > s.b) {
				s.skipInstruction()
				break step
			}
			s.address = Address{}
		case opcodeIFB:
			if !((s.a & s.b) != 0) {
				s.skipInstruction()
				break step
			}
			s.address = Address{}
		case opcodeExtJSR:
			val = s.PC()
			s.DecrSP() // PUSH
			s.address = Address{
				addressType: addressTypeMemory,
				index:       s.SP(),
			}
			s.SetPC(Word(s.a))
		default:
			// cycleCost should have already caught this
			panic("Unexpected opcode")
		}
		if err := s.storeAddress(s.address, val); err != nil {
			s.lastError = err
			return err
		}
		s.step = stateStepFetch
	}
	return nil
}

func decodeOpcode(value Word) (ooooo, aaaaaa, bbbbb uint32) {
	ooooo = uint32(value & 0x1f)
	bbbbb = uint32(value>>5) & 0x1f
	aaaaaa = uint32(value>>10) & 0x3f
	if ooooo == 0 {
		// extended opcode
		ooooo, bbbbb = bbbbb+opcodeExtendedOffset, 0
	}
	return
}

// cycleCost also doubles as an opcode validity test
func cycleCost(opcode uint32) (uint, error) {
	switch opcode {
	case opcodeSET, opcodeAND, opcodeBOR, opcodeXOR:
		return 1, nil
	case opcodeADD, opcodeSUB, opcodeMUL, opcodeSHR, opcodeSHL:
		return 2, nil
	case opcodeDIV, opcodeMOD:
		return 3, nil
	case opcodeIFE, opcodeIFN, opcodeIFG, opcodeIFB:
		return 2, nil
	case opcodeExtJSR:
		return 2, nil
	}
	return 0, &OpcodeError{byte(opcode)}
}

// fetchOperand fetches the value indicated by the operand.
// If the operand needs to fetch the next word and loadWord is false,
// it returns true in delay. Otherwise, if loadWord is true, or if it
// doesn't need to fetch a word, delay will be false and a value will be returned.
// The parameter isB should be set to true for the b operand and false for the a operand.
func (s *State) fetchOperand(operand uint32, loadWord, isB bool) (val Word, address Address, delay bool) {
	switch operand {
	case 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07:
		// register (A, B, C, X, Y, Z, I or J, in that order)
		address = Address{
			addressType: addressTypeRegister,
			index:       Word(operand),
		}
	case 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f:
		// [register]
		address = Address{
			addressType: addressTypeMemory,
			index:       s.Registers[operand-0x08],
		}
	case 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17:
		// [next word + register]
		if loadWord {
			address = Address{
				addressType: addressTypeMemory,
				index:       s.nextWord() + s.Registers[operand-0x10],
			}
		} else {
			delay = true
		}
	case 0x18:
		// (PUSH / [--SP]) if in b, or (POP / [SP++]) if in a
		if isB {
			// PUSH
			s.DecrSP()
			address = Address{
				addressType: addressTypeMemory,
				index:       s.SP(),
			}
		} else {
			// POP
			address = Address{
				addressType: addressTypeMemory,
				index:       s.SP(),
			}
			s.IncrSP()
		}
	case 0x19:
		// PEEK / [SP]
		address = Address{
			addressType: addressTypeMemory,
			index:       s.SP(),
		}
	case 0x1a:
		// [SP + next word] / PICK n
		address = Address{
			addressType: addressTypeMemory,
			index:       s.SP() + s.nextWord(),
		}
	case 0x1b, 0x1c, 0x1d:
		// SP / PC / EX
		// our register indexes go in the same order
		address = Address{
			addressType: addressTypeRegister,
			index:       Word(operand) - 0x1b + registerSP,
		}
	case 0x1e:
		// [next word]
		if loadWord {
			address = Address{
				addressType: addressTypeMemory,
				index:       s.nextWord(),
			}
		} else {
			delay = true
		}
	case 0x1f:
		// next word (literal)
		if loadWord {
			val = s.nextWord()
		} else {
			delay = true
		}
	default:
		if operand > 0x3f {
			// this shouldn't be possible
			panic(fmt.Sprintf("Unexpected operand %#02x", operand))
		}
		// literal value 0xffff-0x1e (-1..30) (literal) (only for a)
		val = Word(operand) - 0x21
	}
	if address.addressType != addressTypeNone {
		val = s.loadAddress(address)
	}
	return
}

// nextWord returns [PC++]
func (s *State) nextWord() Word {
	val := s.Ram.Load(s.PC())
	s.IncrPC()
	return val
}

func (s *State) loadAddress(address Address) Word {
	switch address.addressType {
	case addressTypeNone:
		// we shouldn't be loading this
	case addressTypeRegister:
		return s.Registers[address.index]
	case addressTypeMemory:
		return s.Ram.Load(address.index)
	}
	return 0
}

func (s *State) storeAddress(address Address, value Word) error {
	switch address.addressType {
	case addressTypeNone:
		// do nothing
	case addressTypeRegister:
		s.Registers[address.index] = value
	case addressTypeMemory:
		return s.Ram.Store(address.index, value)
	}
	return nil
}

// skipInstruction sets up the state to execute SET PC, a
// where a is the address of the following instruction
func (s *State) skipInstruction() {
	opcode := s.Ram.Load(s.PC())
	count := instructionLength(opcode)
	s.op = opcodeSET
	s.b = uint32(s.PC() + count)
	s.address = Address{
		addressType: addressTypeRegister,
		index:       registerPC,
	}
	s.cycleCost = 1
}

func instructionLength(opcode Word) Word {
	op, a, b := decodeOpcode(opcode)
	length := 1
	operandCount := func(operand uint32) int {
		if (operand >= 0x10 && operand <= 0x17) || operand == 0x1e || operand == 0x1f {
			return 1
		}
		return 0
	}
	length += operandCount(a)
	if op < opcodeExtendedOffset {
		length += operandCount(b)
	}
	return Word(length)
}

// debugging aids
//
func (a Address) String() string {
	switch a.addressType {
	case addressTypeNone:
		return "<None>"
	case addressTypeRegister:
		reg := []string{"A", "B", "C", "X", "Y", "Z", "I", "J", "PC", "SP", "O"}[a.index]
		return fmt.Sprintf("<%s>", reg)
	case addressTypeMemory:
		return fmt.Sprintf("<[%#02x]>", a.index)
	}
	return "<Unknown>"
}
