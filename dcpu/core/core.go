package core

import (
	"errors"
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
	Ram  Memory
	step func(bool) error
}

type asyncState struct {
	*State
	stepper chan struct{}
	signal  chan error
}

func (s *State) Start() error {
	if s.step != nil {
		return errors.New("State has already started")
	}
	async := asyncState{
		State:   s,
		stepper: make(chan struct{}),
		signal:  make(chan error),
	}
	go func() {
		for {
			if err := async.stepInstruction(); err != nil {
				async.signal <- err
				break
			}
		}
		close(async.signal)
	}()
	var lastError error
	s.step = func(done bool) error {
		if done {
			// if we have an error, the async machine has already stopped
			if lastError == nil {
				close(async.stepper)
				<-async.signal
			}
			return nil
		}
		if lastError != nil {
			return lastError
		}
		async.stepper <- struct{}{}
		lastError = <-async.signal
		return lastError
	}
	// stepInstruction will send out one signal when it's ready.
	// We don't care about waiting until it's ready, but we need to eat that
	// first signal.
	<-async.signal
	return nil
}

func (s *State) StepCycle() error {
	if s.step == nil {
		return errors.New("State has not started")
	}
	return s.step(false)
}

func (s *State) Stop() error {
	if s.step == nil {
		return errors.New("State is already stopped")
	}
	s.step(true)
	s.step = nil
	return nil
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

var errStopped = errors.New("stopped")

func (s *asyncState) readAssignable(assignable assignable) (result Word, ok bool) {
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
func (s *asyncState) writeAssignable(assignable assignable, value Word) (bool, error) {
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

func (s *asyncState) waitCycle() bool {
	s.signal <- nil
	_, ok := <-s.stepper
	return ok
}

func (s *asyncState) nextWord() (Word, bool) {
	// pause for a cycle
	if !s.waitCycle() {
		return 0, false
	}
	word := s.Ram.GetWord(s.PC())
	s.IncrPC()
	return word, true
}

func (s *asyncState) skipInstruction() bool {
	if !s.waitCycle() {
		return false
	}
	s.SetPC(s.PC() + wordCount(s.Ram.GetWord(s.PC())))
	return true
}

func (s *asyncState) translateOperand(op Word) (val Word, assign assignable, ok bool) {
	ok = true
	switch op {
	case 0x0, 0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7:
		// 0x00-0x07: register (A, B, C, X, Y, Z, I or J, in that order)
		assign = assignable{assignableTypeRegister, op}
	case 0x8, 0x9, 0xa, 0xb, 0xc, 0xd, 0xe, 0xf:
		// 0x08-0x0f: [register]
		assign = assignable{assignableTypeMemory, s.Registers[op-8]}
	case 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17:
		// 0x10-0x17: [next word + register]
		// note: takes 1 cycle
		var w Word
		if w, ok = s.nextWord(); !ok {
			return
		}
		assign = assignable{assignableTypeMemory, w + s.Registers[op-16]}
	case 0x18:
		// 0x18: POP / [SP++]
		assign = assignable{assignableTypeMemory, s.SP()}
		s.IncrSP()
	case 0x19:
		// 0x19: PEEK / [SP]
		assign = assignable{assignableTypeMemory, s.SP()}
	case 0x1a:
		// 0x1a: PUSH / [--SP]
		s.DecrSP()
		assign = assignable{assignableTypeMemory, s.SP()}
	case 0x1b:
		// 0x1b: SP
		assign = assignable{assignableTypeRegister, registerSP}
	case 0x1c:
		// 0x1c: PC
		assign = assignable{assignableTypeRegister, registerPC}
	case 0x1d:
		// 0x1d: O
		assign = assignable{assignableTypeRegister, registerO}
	case 0x1e:
		// 0x1e: [next word]
		// note: takes 1 cycle
		var w Word
		if w, ok = s.nextWord(); !ok {
			return
		}
		assign = assignable{assignableTypeMemory, w}
	case 0x1f:
		// 0x1f: next word (literal)
		// note: takes 1 cycle
		var w Word
		if w, ok = s.nextWord(); !ok {
			return
		}
		val = w
	default:
		// 0x20-0x3f: literal value 0x00-0x1f (literal)
		if op > 0x3f {
			panic("Out of bounds operand")
		}
		val = op - 0x20
	}
	if assVal, ok := s.readAssignable(assign); ok {
		val = assVal
	}
	return
}

func (s *asyncState) stepInstruction() error {
	// fetch
	opcode, ok := s.nextWord()
	if !ok {
		return errStopped
	}

	// decode
	ins, a, b := decodeOpcode(opcode)

	var assign assignable
	if ins != 0 { // don't translate for the non-basic opcodes
		if a, assign, ok = s.translateOperand(a); !ok {
			return errStopped
		}
		if b, _, ok = s.translateOperand(b); !ok {
			return errStopped
		}
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
			// 2 cycles
			if !s.waitCycle() {
				return errStopped
			}
			if _, assign, ok = s.translateOperand(0x1a); !ok { // PUSH
				return errStopped
			}
			if a, _, ok = s.translateOperand(a); !ok {
				return errStopped
			}
			s.writeAssignable(assign, s.PC())
			s.SetPC(a)
			assign = assignable{}
		default:
			return &OpcodeError{opcode}
		}
	case 1:
		// SET a, b - sets a to b
		// 1 cycle
		val = b
	case 2:
		// ADD a, b - sets a to a+b, sets O to 0x0001 if there's an overflow, 0x0 otherwise
		// 2 cycles
		if !s.waitCycle() {
			return errStopped
		}
		result := uint32(a) + uint32(b)
		val = Word(result & 0xFFFF)
		s.SetO(Word(result >> 16)) // will always be 0x0 or 0x1
	case 3:
		// SUB a, b - sets a to a-b, sets O to 0xffff if there's an underflow, 0x0 otherwise
		// 2 cycles
		if !s.waitCycle() {
			return errStopped
		}
		result := uint32(a) - uint32(b)
		val = Word(result & 0xFFFF)
		s.SetO(Word(result >> 16)) // will always be 0x0 or 0xffff
	case 4:
		// MUL a, b - sets a to a*b, sets O to ((a*b)>>16)&0xffff
		// 2 cycles
		if !s.waitCycle() {
			return errStopped
		}
		result := uint32(a) * uint32(b)
		val = Word(result & 0xFFFF)
		s.SetO(Word(result >> 16))
	case 5:
		// DIV a, b - sets a to a/b, sets O to ((a<<16)/b)&0xffff. if b==0, sets a and O to 0 instead.
		// 3 cycles
		if !s.waitCycle() || !s.waitCycle() {
			return errStopped
		}
		if b == 0 {
			val = 0
			s.SetO(0)
		} else {
			val = a / b
			s.SetO(Word(((uint32(a) << 16) / uint32(b))))
		}
	case 6:
		// MOD a, b - sets a to a%b. if b==0, sets a to 0 instead.
		// 3 cycles
		if !s.waitCycle() || !s.waitCycle() {
			return errStopped
		}
		if b == 0 {
			val = 0
		} else {
			val = a % b
		}
	case 7:
		// SHL a, b - sets a to a<<b, sets O to ((a<<b)>>16)&0xffff
		// 2 cycles
		if !s.waitCycle() {
			return errStopped
		}
		result := uint32(a) << uint32(b)
		val = Word(result & 0xFFFF)
		s.SetO(Word(result >> 16))
	case 8:
		// SHR a, b - sets a to a>>b, sets O to ((a<<16)>>b)&0xffff
		// 2 cycles
		if !s.waitCycle() {
			return errStopped
		}
		val = a >> b
		s.SetO(Word((uint32(a) << 16) >> b))
	case 9:
		// AND a, b - sets a to a&b
		// 1 cycle
		val = a & b
	case 10:
		// BOR a, b - sets a to a|b
		// 1 cycle
		val = a | b
	case 11:
		// XOR a, b - sets a to a^b
		// 1 cycle
		val = a ^ b
	case 12:
		// IFE a, b - performs next instruction only if a==b
		// 2 cycles +1
		if !s.waitCycle() {
			return errStopped
		}
		if a != b {
			if !s.skipInstruction() {
				return errStopped
			}
		}
		assign = assignable{}
	case 13:
		// IFN a, b - performs next instruction only if a!=b
		// 2 cycles +1
		if !s.waitCycle() {
			return errStopped
		}
		if a == b {
			if !s.skipInstruction() {
				return errStopped
			}
		}
		assign = assignable{}
	case 14:
		// IFG a, b - performs next instruction only if a>b
		// 2 cycles +1
		if !s.waitCycle() {
			return errStopped
		}
		if a <= b {
			if !s.skipInstruction() {
				return errStopped
			}
		}
		assign = assignable{}
	case 15:
		// IFB a, b - performs next instruction only if (a&b)!=0
		// 2 cycles +1
		if !s.waitCycle() {
			return errStopped
		}
		if (a & b) == 0 {
			if !s.skipInstruction() {
				return errStopped
			}
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
