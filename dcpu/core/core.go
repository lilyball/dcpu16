package core

import (
	"errors"
	"fmt"
)

type Word uint16

type OpcodeError struct {
	Opcode
}

var HaltError = errors.New("Halt and Catch Fire")

func (err *OpcodeError) Error() string {
	return fmt.Sprintf("invalid opcode %#04x", err.Opcode)
}

type State struct {
	Registers
	Ram       Memory
	lastError error   // once set, will be returned always
	step      int     // fetch, decode, execute
	cycleCost uint    // remaining cost of the opcode to execute
	op        Opcode  // opcode
	a, b      uint32  // operands (uint32 datatype used for math)
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
		// Test for a pending interrupt
		// Comment this out for now. It requires a slight rethink. Not useful
		// until we actually have hardware anyway.
		/*if len(s.interrupts) > 0 {
			message := s.interrupts[0]
			s.interrupts = s.interrupts[1:]
			// shove an INT instruction into our state
			s.op = opcodeExtINT
			s.a = uint32(message)
			if cost, err := cycleCost(s.op); err != nil {
				panic("Unexpected error from cycleCost for opcodeExtINT")
			} else {
				s.cycleCost = cost
			}
			s.step = stateStepExecute // no decoding needed
			goto step                 // restart the cycle
		}*/
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
		if s.op >= opcodeExtendedOffset {
			s.address = loc
			s.step = stateStepExecute
		} else {
			s.step = stateStepDecodeB
		}
		fallthrough
	case stateStepDecodeB:
		// decode operand B
		val, loc, delay := s.fetchOperand(s.b, s.delayed, true)
		s.delayed = delay
		if delay {
			break
		}
		s.b = uint32(val)
		s.address = loc
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
			val = Word(s.a)
		case opcodeADD:
			result := s.b + s.a
			val = Word(result)
			s.SetEX(Word(result >> 16))
		case opcodeSUB:
			result := s.b - s.a
			val = Word(result)
			s.SetEX(Word(result >> 16))
		case opcodeMUL:
			result := s.b * s.a
			val = Word(result)
			s.SetEX(Word(result >> 16))
		case opcodeMLI:
			result := int32(s.b) * int32(s.a)
			val = Word(result)
			s.SetEX(Word(result >> 16))
		case opcodeDIV:
			if s.a == 0 {
				val = 0
				s.SetEX(0)
			} else {
				result := s.b / s.a
				val = Word(result)
				// EX is a bit weird here
				s.SetEX(Word((s.b << 16) / s.a))
			}
		case opcodeDVI:
			if s.a == 0 {
				val = 0
				s.SetEX(0)
			} else {
				result := int16(s.b) / int16(s.a)
				val = Word(result)
				// EX is a bit weird here
				s.SetEX(Word((int32(s.b) << 16) / int32(s.a)))
			}
		case opcodeMOD:
			if s.a == 0 {
				val = 0
			} else {
				val = Word(s.b % s.a)
			}
		case opcodeAND:
			val = Word(s.b & s.a)
		case opcodeBOR:
			val = Word(s.b | s.a)
		case opcodeXOR:
			val = Word(s.b ^ s.a)
		case opcodeSHR:
			result := (s.b << 16) >> s.a
			val = Word(result >> 16)
			s.SetEX(Word(result))
		case opcodeASR:
			result := (int32(s.b) << 16) >> s.a
			val = Word(result >> 16)
			s.SetEX(Word(result))
		case opcodeSHL:
			result := s.a << s.b
			val = Word(result)
			s.SetEX(Word(result >> 16))
		case opcodeSTI:
			val = Word(s.a)
			s.SetI(s.I() + 1)
			s.SetJ(s.J() + 1)
		case opcodeIFB, opcodeIFC, opcodeIFE, opcodeIFN, opcodeIFG, opcodeIFA, opcodeIFL, opcodeIFU:
			var test bool
			switch s.op {
			case opcodeIFB:
				test = (s.b & s.a) != 0
			case opcodeIFC:
				test = (s.b & s.a) == 0
			case opcodeIFE:
				test = s.b == s.a
			case opcodeIFN:
				test = s.b != s.a
			case opcodeIFG:
				test = s.b > s.a
			case opcodeIFA:
				test = int16(s.b) > int16(s.a)
			case opcodeIFL:
				test = s.b < s.a
			case opcodeIFU:
				test = int16(s.b) < int16(s.a)
			}
			if !test {
				s.skipInstruction()
				break step
			}
			s.address = Address{}
		case opcodeADX:
			result := s.b + s.a + uint32(s.EX())
			val = Word(result)
			s.SetEX(Word(result >> 16))
		case opcodeSBX:
			result := s.b - s.a + uint32(s.EX())
			val = Word(result)
			s.SetEX(Word(result >> 16))
		/* extended opcodes */
		case opcodeExtJSR:
			val = s.PC()
			s.DecrSP() // PUSH
			s.address = Address{
				addressType: addressTypeMemory,
				index:       s.SP(),
			}
			s.SetPC(Word(s.a))
		case opcodeExtHCF:
			// there's no documentation on what this does, but presumably it halts the machine.
			err := HaltError
			s.lastError = err
			return err
		case opcodeExtINT:
			// Note: if hardware is really allowed to modify registers outside of
			// a hardware interrupt, then this needs to be rewritten to use the 4 cycles
			// to write the registers independently, instead of dumping them all at once here.
			message := s.a
			// re-use the cycle machinery to make writing to memory a bit easier
			// temporarily disable interrupts
			/*interrupts := s.interrupts
			s.interrupts = nil*/
			// SET PUSH, PC
			s.op = opcodeSET
			s.b = operandPushPop
			s.a = operandPC
			s.cycleCost = 0
			s.step = stateStepDecodeA
			if err := s.StepCycle(); err != nil {
				return err
			}
			// SET PUSH, A
			s.op = opcodeSET
			s.b = operandPushPop
			s.a = operandA
			s.cycleCost = 0
			s.step = stateStepDecodeA
			if err := s.StepCycle(); err != nil {
				return err
			}
			// SET A, message
			s.SetA(Word(message))
			// SET PC, IA
			s.SetPC(s.IA())
			s.address = Address{}
			// re-enable interrupts
			/*s.interrupts = interrupts*/
		case opcodeExtIAG:
			val = s.IA()
		case opcodeExtIAS:
			val = Word(s.a)
			s.address = Address{
				addressType: addressTypeRegister,
				index:       registerIA,
			}
		case opcodeExtHWN:
			// hardware support is forthcoming
			val = 0
		case opcodeExtHWQ:
			// hardware support is forthcoming
			// it's undefined in the spec, but I assume that an out-of-bounds hardware request
			// will just set everything to 0
			s.SetA(0)
			s.SetB(0)
			s.SetC(0)
			s.SetX(0)
			s.SetY(0)
			s.address = Address{}
		case opcodeExtHWI:
			// hardware support is forthcoming
			s.address = Address{}
		default:
			// cycleCost should have already caught this
			panic(fmt.Sprintf("Unexpected opcode %#04x", s.op))
		}
		if err := s.storeAddress(s.address, val); err != nil {
			s.lastError = err
			return err
		}
		s.step = stateStepFetch
	}
	return nil
}

func decodeOpcode(value Word) (ooooo Opcode, aaaaaa, bbbbb uint32) {
	ooooo = Opcode(value & 0x1f)
	bbbbb = uint32(value>>5) & 0x1f
	aaaaaa = uint32(value>>10) & 0x3f
	if ooooo == 0 {
		// extended opcode
		ooooo, bbbbb = Opcode(bbbbb+opcodeExtendedOffset), 0
	}
	return
}

var cycleCostMap = map[Opcode]uint{
	opcodeSET: 1,
	opcodeADD: 2, opcodeSUB: 2,
	opcodeMUL: 2, opcodeMLI: 2,
	opcodeDIV: 3, opcodeDVI: 3, opcodeMOD: 3,
	opcodeAND: 1, opcodeBOR: 1, opcodeXOR: 1,
	opcodeSHR: 2, opcodeASR: 2, opcodeSHL: 2,
	opcodeSTI: 2,
	opcodeIFB: 2, opcodeIFC: 2, opcodeIFE: 2, opcodeIFN: 2,
	opcodeIFG: 2, opcodeIFA: 2, opcodeIFL: 2, opcodeIFU: 2,
	opcodeADX: 3, opcodeSBX: 3,
	opcodeExtJSR: 3,
	opcodeExtHCF: 9,
	opcodeExtINT: 4, opcodeExtIAG: 1, opcodeExtIAS: 1,
	opcodeExtHWN: 2, opcodeExtHWQ: 4, opcodeExtHWI: 4,
}

// cycleCost also doubles as an opcode validity test
func cycleCost(opcode Opcode) (uint, error) {
	if cost, ok := cycleCostMap[opcode]; ok {
		return cost, nil
	}
	return 0, &OpcodeError{opcode}
}

// fetchOperand fetches the value indicated by the operand.
// If the operand needs to fetch the next word and loadWord is false,
// it returns true in delay. Otherwise, if loadWord is true, or if it
// doesn't need to fetch a word, delay will be false and a value will be returned.
// The parameter isB should be set to true for the b operand and false for the a operand.
func (s *State) fetchOperand(operand uint32, loadWord, isB bool) (val Word, address Address, delay bool) {
	switch operand {
	case operandA /* 0x00 */, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07:
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
	case operandPushPop: // 0x18
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
		if loadWord {
			address = Address{
				addressType: addressTypeMemory,
				index:       s.SP() + s.nextWord(),
			}
		} else {
			delay = true
		}
	case 0x1b, operandPC /* 0x1c */, 0x1d:
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
		} else if operand < 0x20 {
			// this also shouldn't be possible
			panic(fmt.Sprintf("Program error: didn't handle operand %#02x", operand))
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
	count := Word(0)
	cost := uint(0)
	for {
		opcode := s.Ram.Load(s.PC() + count)
		count += instructionLength(opcode)
		cost++
		op, _, _ := decodeOpcode(opcode)
		if op >= opcodeIFB && op <= opcodeIFU {
			// it's a chained if
		} else {
			break
		}
	}
	s.op = opcodeSET
	s.a = uint32(s.PC() + count)
	s.address = Address{
		addressType: addressTypeRegister,
		index:       registerPC,
	}
	s.cycleCost = cost
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
		reg := []string{"A", "B", "C", "X", "Y", "Z", "I", "J", "PC", "SP", "EX", "IA"}[a.index]
		return fmt.Sprintf("<%s>", reg)
	case addressTypeMemory:
		return fmt.Sprintf("<[%#02x]>", a.index)
	}
	return "<Unknown>"
}
