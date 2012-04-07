package core

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
	Ram        Memory
	lastError  error // once set, will be returned always
	tasks      []task
	a, b, op   Word
	cycleCount int // cycle count for op
	assign     assignable
}

const maxOperandTasks = 3

type task struct {
	taskType   int
	cycleCost  int // extra cycle cost for this task
	value      int
	result     *Word
	markAssign bool
}

const (
	taskTypeNone     = iota
	taskTypeFetch    // ignores value, fetches current *result
	taskTypeRegister // adds register (value) to *result
	taskTypeNextWord // fetches the next word into *result and increments PC
	taskTypePushPop  // increments/decrements SP by value, ignores result
)

func (s *State) StepCycle() error {
	if s.lastError != nil {
		return s.lastError
	}
	// check if our task list is populated
	if len(s.tasks) == 0 {
		if err := s.constructTaskList(); err != nil {
			s.lastError = err
			return err
		}
	}
	// process our tasks
	for i := 0; i < len(s.tasks); i++ {
		task := &s.tasks[i]
		if task.cycleCost > 0 {
			task.cycleCost--
			return nil
		}
		switch task.taskType {
		case taskTypeNone:
			continue
		case taskTypeFetch:
			if task.markAssign {
				s.assign = assignable{
					valueType: assignableTypeMemory,
					index:     *task.result,
				}
			}
			*task.result = s.Ram.GetWord(*task.result)
		case taskTypeRegister:
			if task.markAssign {
				s.assign = assignable{
					valueType: assignableTypeRegister,
					index:     Word(task.value),
				}
			}
			*task.result += s.Registers[task.value]
		case taskTypeNextWord:
			// this can't mark assign
			*task.result = s.nextWord()
		case taskTypePushPop:
			if task.value > 0 {
				s.SetSP(s.SP() + Word(task.value))
			} else {
				s.SetSP(s.SP() - Word(-task.value))
			}
		default:
			panic("unknown task type")
		}
		task.taskType = taskTypeNone
	}
	if s.cycleCount > 1 {
		s.cycleCount--
		return nil
	}
	// clear our tasks
	s.tasks = s.tasks[:0]
	// execute the operation
	var val Word
	switch s.op {
	case opcodeSET:
		// SET a, b - sets a to b
		val = s.b
	case opcodeADD:
		// ADD a, b - sets a to a+b, sets O to 0x0001 if there's an overflow, 0x0 otherwise
		result := uint32(s.a) + uint32(s.b)
		val = Word(result & 0xFFFF)
		s.SetO(Word(result >> 16))
	case opcodeSUB:
		// SUB a, b - sets a to a-b, sets O to 0xffff if there's an underflow, 0x0 otherwise
		result := uint32(s.a) - uint32(s.b)
		val = Word(result & 0xFFFF)
		s.SetO(Word(result >> 16))
	case opcodeMUL:
		// MUL a, b - sets a to a*b, sets O to ((a*b)>>16)&0xffff
		result := uint32(s.a) * uint32(s.b)
		val = Word(result & 0xFFFF)
		s.SetO(Word(result >> 16))
	case opcodeDIV:
		// DIV a, b - sets a to a/b, sets O to ((a<<16)/b)&0xffff. if b==0, sets a and O to 0 instead.
		if s.b == 0 {
			val = 0
			s.SetO(0)
		} else {
			val = s.a / s.b
			s.SetO(Word(((uint32(s.a) << 16) / uint32(s.b))))
		}
	case opcodeMOD:
		// MOD a, b - sets a to a%b. if b==0, sets a to 0 instead.
		if s.b == 0 {
			val = 0
		} else {
			val = s.a % s.b
		}
	case opcodeSHL:
		// SHL a, b - sets a to a<<b, sets O to ((a<<b)>>16)&0xffff
		result := uint32(s.a) << uint32(s.b)
		val = Word(result & 0xFFFF)
		s.SetO(Word(result >> 16))
	case opcodeSHR:
		// SHR a, b - sets a to a>>b, sets O to ((a<<16)>>b)&0xffff
		val = s.a >> s.b
		s.SetO(Word(uint32(s.a)<<16) >> s.b)
	case opcodeAND:
		// AND a, b - sets a to a&b
		val = s.a & s.b
	case opcodeBOR:
		// BOR a, b - sets a to a|b
		val = s.a | s.b
	case opcodeXOR:
		// XOR a, b - sets a to a^b
		val = s.a ^ s.b
	case opcodeIFE:
		// IFE a, b - performs next instruction only if a==b
		if s.a != s.b {
			s.skipInstruction()
		}
		s.assign = assignable{}
	case opcodeIFN:
		// IFN a, b - performs next instruction only if a!=b
		if s.a == s.b {
			s.skipInstruction()
		}
		s.assign = assignable{}
	case opcodeIFG:
		// IFG a, b - performs next instruction only if a>b
		if s.a <= s.b {
			s.skipInstruction()
		}
		s.assign = assignable{}
	case opcodeIFB:
		// IFB a, b - performs next instruction only if (a&b)!=0
		if (s.a & s.b) == 0 {
			s.skipInstruction()
		}
		s.assign = assignable{}
	case opcodeExtJSR:
		// JSR a - pushes the address of the next instruction to the stack, then sets PC to a
		s.DecrSP()
		s.Ram.SetWord(s.SP(), s.PC())
		val = s.a
		s.assign = assignable{assignableTypeRegister, registerPC}
	default:
		// unknown but in-bounds opcodes are detected in constructTaskList()
		panic("Out of bounds opcode")
	}

	if _, err := s.writeAssignable(s.assign, val); err != nil {
		s.lastError = err
		return s.lastError
	}
	return nil
}

func (s *State) constructTaskList() error {
	// fill out our task list
	if s.tasks == nil {
		// push our task list up to the max cap immediately
		s.tasks = make([]task, 0, maxOperandTasks*2)
	} else {
		s.tasks = s.tasks[:0]
	}
	opcode := s.nextWord()
	op, a, b := decodeOpcode(opcode)
	if op == 0 {
		// non-basic opcode
		op, a = 0x100+a, b
	}
	s.op = op
	s.a = 0
	s.b = 0
	for i := 0; i < 2; i++ {
		var val Word
		var result *Word
		var assign bool
		if i == 0 {
			val = a
			result = &s.a
			assign = true
		} else if op >= 0x100 {
			// this is an extended opcode. They don't have bbbbbb
			break
		} else {
			val = b
			result = &s.b
		}
		switch val {
		case 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07:
			// register (A, B, C, X, Y, Z, I or J, in that order)
			s.tasks = append(s.tasks, task{
				taskType:   taskTypeRegister,
				value:      int(val),
				result:     result,
				markAssign: assign,
			})
		case 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f:
			// [register]
			s.tasks = append(s.tasks, task{
				taskType: taskTypeRegister,
				value:    int(val - 0x08),
				result:   result,
			})
			s.tasks = append(s.tasks, task{
				taskType:   taskTypeFetch,
				result:     result,
				markAssign: assign,
			})
		case 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17:
			// [next word + register]
			s.tasks = append(s.tasks, task{
				taskType:  taskTypeNextWord,
				cycleCost: 1,
				result:    result,
			})
			s.tasks = append(s.tasks, task{
				taskType: taskTypeRegister,
				value:    int(val - 0x10),
				result:   result,
			})
			s.tasks = append(s.tasks, task{
				taskType:   taskTypeFetch,
				result:     result,
				markAssign: assign,
			})
		case 0x18:
			// POP / [SP++]
			s.tasks = append(s.tasks, task{
				taskType: taskTypeRegister,
				value:    registerSP,
				result:   result,
			})
			s.tasks = append(s.tasks, task{
				taskType:   taskTypeFetch,
				result:     result,
				markAssign: assign,
			})
			s.tasks = append(s.tasks, task{
				taskType: taskTypePushPop,
				value:    1,
			})
		case 0x19:
			// PEEK / [SP]
			s.tasks = append(s.tasks, task{
				taskType: taskTypeRegister,
				value:    registerSP,
				result:   result,
			})
			s.tasks = append(s.tasks, task{
				taskType:   taskTypeFetch,
				result:     result,
				markAssign: assign,
			})
		case 0x1a:
			// PUSH / [--SP]
			s.tasks = append(s.tasks, task{
				taskType: taskTypePushPop,
				value:    -1,
			})
			s.tasks = append(s.tasks, task{
				taskType: taskTypeRegister,
				value:    registerSP,
				result:   result,
			})
			s.tasks = append(s.tasks, task{
				taskType:   taskTypeFetch,
				result:     result,
				markAssign: assign,
			})
		case 0x1b:
			// SP
			s.tasks = append(s.tasks, task{
				taskType:   taskTypeRegister,
				value:      registerSP,
				result:     result,
				markAssign: assign,
			})
		case 0x1c:
			// PC
			s.tasks = append(s.tasks, task{
				taskType:   taskTypeRegister,
				value:      registerPC,
				result:     result,
				markAssign: assign,
			})
		case 0x1d:
			// O
			s.tasks = append(s.tasks, task{
				taskType:   taskTypeRegister,
				value:      registerO,
				result:     result,
				markAssign: assign,
			})
		case 0x1e:
			// [next word]
			s.tasks = append(s.tasks, task{
				taskType:  taskTypeNextWord,
				cycleCost: 1,
				result:    result,
			})
			s.tasks = append(s.tasks, task{
				taskType:   taskTypeFetch,
				result:     result,
				markAssign: assign,
			})
		case 0x1f:
			// next word (literal)
			s.tasks = append(s.tasks, task{
				taskType:  taskTypeNextWord,
				cycleCost: 1,
				result:    result,
			})
		default:
			if val <= 0x3f {
				*result = val - 0x20
			} else {
				panic("Out of bounds operand")
			}
		}
	}
	switch s.op {
	case opcodeSET, opcodeAND, opcodeBOR, opcodeXOR:
		s.cycleCount = 1
	case opcodeADD, opcodeSUB, opcodeMUL, opcodeSHR, opcodeSHL:
		s.cycleCount = 2
	case opcodeDIV, opcodeMOD:
		s.cycleCount = 3
	case opcodeIFE, opcodeIFN, opcodeIFG, opcodeIFB:
		s.cycleCount = 2
	case opcodeExtJSR:
		s.cycleCount = 2
	default:
		return &OpcodeError{opcode}
	}
	return nil
}

// constructs a new task list for SET PC, a
func (s *State) skipInstruction() {
	s.tasks = s.tasks[:0]
	s.tasks = append(s.tasks, task{
		taskType:   taskTypeRegister,
		value:      registerPC,
		result:     &s.a,
		markAssign: true,
	})
	opcode := s.Ram.GetWord(s.PC())
	s.b = s.PC() + wordCount(opcode)
	s.op = opcodeSET
	s.cycleCount = 1
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

func (a assignable) String() string {
	switch a.valueType {
	case assignableTypeNone:
		return "(literal)"
	case assignableTypeRegister:
		switch a.index {
		case registerA:
			return "A"
		case registerB:
			return "B"
		case registerC:
			return "C"
		case registerX:
			return "X"
		case registerY:
			return "Y"
		case registerZ:
			return "Z"
		case registerI:
			return "I"
		case registerJ:
			return "J"
		case registerSP:
			return "SP"
		case registerPC:
			return "PC"
		case registerO:
			return "O"
		default:
			return "(unknown register)"
		}
	case assignableTypeMemory:
		return fmt.Sprintf("[%#04x]", a.index)
	}
	return "(unknown)"
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
