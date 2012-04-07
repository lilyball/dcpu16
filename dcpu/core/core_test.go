package core

import (
	"testing"
)

func TestLoadProgram(t *testing.T) {
	state := new(State)
	if err := state.LoadProgram(notchAssemblerTestProgram[:], 0); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < len(notchAssemblerTestProgram); i++ {
		if state.Ram.GetWord(Word(i)) != notchAssemblerTestProgram[i] {
			t.Errorf("Expected word %04x, found word %04x at offset %d", notchAssemblerTestProgram[i], state.Ram.GetWord(Word(i)), i)
			break
		}
	}
}

func TestNotchAssemblerTest(t *testing.T) {
	state := new(State)
	if err := state.LoadProgram(notchAssemblerTestProgram[:], 0); err != nil {
		t.Fatal(err)
	}

	// step the program for 1000 cycles, or until it hits the opcode 0x85C3
	// hitting 1000 cycles is considered failure
	for i := 0; i < 1000; i++ {
		t.Logf("%d: %04x", state.PC(), state.Ram.GetWord(state.PC()))
		if err := state.StepCycle(); err != nil {
			t.Fatal(err)
			break
		}
		if state.Ram.GetWord(state.PC()) == 0x85C3 { // sub PC, 1
			break
		}
	}
	if state.Ram.GetWord(state.PC()) != 0x85C3 {
		// we exhausted our steps
		t.Error("Program exceeded 1000 cycles")
	}
	// check 0x8000 - 0x800B for "Hello world!"
	expected := "Hello world!"
	for i := 0; i < len(expected); i++ {
		if state.Ram.GetWord(Word(0x8000+i)) != Word(expected[i]) {
			t.Errorf("Unexpected output in video ram; expected %v, found %v", []byte(expected), state.Ram.GetSlice(0x8000, 0x800B))
			break
		}
	}
}

var notchAssemblerTestProgram = [...]Word{
	//              set a, 0xbeef
	0x7C01, // 0
	0xBEEF, // 1
	//              set [0x1000], a
	0x01E1, // 2
	0x1000, // 3
	//              ifn a, [0x1000]
	0x780D, // 4
	0x1000, // 5
	//                  set PC, end
	0x7DC1, // 6
	32,     // 7
	//
	//              set i, 0
	0x8061, // 8
	// :nextchar    ife [data+i], 0
	0x816C, // 9
	19,     // 10
	//                  set PC, end
	0x7DC1, // 11
	32,     // 12
	//              set [0x8000+i], [data+i]
	0x5961, // 13
	0x8000, // 14
	19,     // 15
	//              add i, 1
	0x8462, // 16
	//              set PC, nextchar
	0x7DC1, // 17
	9,      // 18
	//
	// :data        dat "Hello world!", 0
	'H', 'e', 'l', 'l', 'o', ' ', 'w', 'o', 'r', 'l', 'd', '!', 0, // 19-31
	//
	// :end         sub PC, 1
	0x85C3, // 32
}

func TestNotchSpecExample(t *testing.T) {
	state := new(State)
	if err := state.LoadProgram(notchSpecExampleProgram[:], 0); err != nil {
		t.Fatal(err)
	}

	// test the first section
	for i := 0; i < 11; i++ {
		if err := state.StepCycle(); err != nil {
			t.Fatal(err)
		}
	}
	if len(state.tasks) != 0 {
		t.Errorf("Unexpectedly stopped mid-instruction")
	}
	if state.A() != 0x10 {
		t.Errorf("Unexpected value for register A; expected %#x, found %#x", 0x10, state.A())
	}
	if state.PC() != 10 {
		t.Errorf("Unexpected value for register PC; expected %#x, found %#x", 10, state.A())
	}
	// run 23 more cycles (12 more instructions, partway into the loop)
	for i := 0; i < 23; i++ {
		if err := state.StepCycle(); err != nil {
			t.Fatal(err)
		}
	}
	if len(state.tasks) != 0 {
		t.Errorf("Unexpectedly stopped mid-instruction")
	}
	if state.I() != 7 {
		t.Errorf("Unexpected value for register I; expected %#x, found %#x", 7, state.I())
	}
	if state.PC() != 16 {
		t.Errorf("Unexpected value for register PC; expected %#x, found %#x", 16, state.PC())
	}
	// 59 more cycles (29 more instructions) to finish the loop
	for i := 0; i < 59; i++ {
		if err := state.StepCycle(); err != nil {
			t.Fatal(err)
		}
	}
	if state.I() != 0 {
		t.Errorf("Unexpected value for register I; expected %#x, found %#x", 0, state.I())
	}
	if state.PC() != 19 {
		t.Errorf("Unexpected value for register PC; expected %#x, found t#x", 19, state.PC())
	}
	if state.SP() != 0 {
		t.Errorf("Unexpected value for register SP; expected %#x, found %#x", 0, state.SP())
	}
	// 4 more cycles (2 more instructions) to put us into the subroutine
	for i := 0; i < 4; i++ {
		if err := state.StepCycle(); err != nil {
			t.Fatal(err)
		}
	}
	if state.X() != 4 {
		t.Errorf("Unexpected value for register X; expected %#x, found %#x", 4, state.X())
	}
	if state.PC() != 24 {
		t.Errorf("Unexpected value for register PC; expected %#x, found %#x", 24, state.PC())
	}
	if state.SP() != 0xffff {
		t.Errorf("Unexpected value for register SP; expected %#x, found %#x", 0xffff, state.SP())
	}
	if state.Ram.GetWord(0xffff) != 22 {
		t.Errorf("Unexpected value at 0xffff; expected %#x, found %#x", 22, state.Ram.GetWord(0xffff))
		t.FailNow()
	}
	if t.Failed() {
		t.FailNow()
	}
	// run the program for 1000 cycles, or until it hits the instruction 0x7DC1 PC
	success := false
	for i := 0; i < 1000; i++ {
		if err := state.StepCycle(); err != nil {
			t.Fatal(err)
		}
		if state.Ram.GetWord(state.PC()) == 0x7DC1 && state.Ram.GetWord(state.PC()+1) == state.PC() {
			success = true
			break
		}
	}
	if !success {
		// we exhausted our steps
		t.Error("Program exceeded 1000 cycles")
	}

	// Check register X, it should be 0x40
	if state.X() != 0x40 {
		t.Error("Unexpected value for register X; expected %#x, found %#x", 0x40, state.X())
	}
}

var notchSpecExampleProgram = [...]Word{
	0x7c01, 0x0030, 0x7de1, 0x1000, 0x0020, 0x7803, 0x1000, 0xc00d,
	0x7dc1, 0x001a, 0xa861, 0x7c01, 0x2000, 0x2161, 0x2000, 0x8463,
	0x806d, 0x7dc1, 0x000d, 0x9031, 0x7c10, 0x0018, 0x7dc1, 0x001a,
	0x9037, 0x61c1, 0x7dc1, 0x001a, 0x0000, 0x0000, 0x0000, 0x0000,
}

func TestMemoryMappedIO(t *testing.T) {
	state := new(State)
	if err := state.LoadProgram(notchAssemblerTestProgram[:], 0); err != nil {
		t.Fatal(err)
	}
	// set up the mappings
	buffer := [0x400]Word{}
	get := func(address Word) Word {
		return buffer[address]
	}
	set := func(address, val Word) error {
		buffer[address] = val
		return nil
	}
	if err := state.Ram.MapRegion(0x8000, 0x400, get, set); err != nil {
		t.Fatal(err)
	}
	// run the program for up to 1000 cycles, or until it hits opcode 0x85C3
	for i := 0; i < 1000; i++ {
		if err := state.StepCycle(); err != nil {
			t.Fatal(err)
		}
		if state.Ram.GetWord(state.PC()) == 0x85C3 { // sub PC, 1
			break
		}
	}
	expected := "Hello world!"
	for i := 0; i < len(expected); i++ {
		if buffer[i] != Word(expected[i]) {
			t.Errorf("Unexpected output in video ram; expected %v, found %v", []byte(expected), buffer[:len(expected)])
			break
		}
	}
	for i := len(expected); i < len(buffer); i++ {
		w := buffer[i]
		if w != 0 {
			t.Errorf("Unexpected output in video ram at offset %#v; expected 0x0, found %#x", i, w)
			break
		}
	}
}
