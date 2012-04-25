package core

import (
	"fmt"
	"testing"
)

func TestLoadProgram(t *testing.T) {
	state := new(State)
	if err := state.LoadProgram(notchAssemblerTestProgram[:], 0); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < len(notchAssemblerTestProgram); i++ {
		if state.Ram.Load(Word(i)) != notchAssemblerTestProgram[i] {
			t.Errorf("Expected word %04x, found word %04x at offset %d", notchAssemblerTestProgram[i], state.Ram.Load(Word(i)), i)
			break
		}
	}
}

func logPC(t *testing.T, state *State) {
	extra := ""
	pc := state.PC()
	word := state.Ram.Load(pc)
	if state.step == stateStepFetch {
		op, a, b := decodeOpcode(word)
		extra = fmt.Sprintf("(fetch: %#02x %#02x %#02x)", op, b, a)
	}
	t.Logf("%#02x: %#04x %s", pc, word, extra)
}

func logRegisters(t *testing.T, state *State) {
	t.Logf("A: %#02x  B: %#02x  C: %#02x", state.A(), state.B(), state.C())
	t.Logf("X: %#02x  Y: %#02x  Z: %#02x", state.X(), state.Y(), state.Z())
	t.Logf("I: %#02x  J: %#02x", state.I(), state.J())
}

func cycle(t *testing.T, state *State) error {
	logPC(t, state)
	return state.StepCycle()
}

func TestNotchAssemblerTest(t *testing.T) {
	state := new(State)
	if err := state.LoadProgram(notchAssemblerTestProgram[:], 0); err != nil {
		t.Fatal(err)
	}

	// Step the program twice, checking the PC each time
	// We want to ensure the PC is incremented only by 1 each time, instead of incrementing
	// by 2 on the first cycle
	if err := cycle(t, state); err != nil {
		t.Fatal(err)
	}
	if state.PC() != 0x1 {
		t.Errorf("Unexpected value for PC; expected %#02x, found %#02x", 0x1, state.PC())
	}
	if err := cycle(t, state); err != nil {
		t.Fatal(err)
	}
	if state.PC() != 0x2 {
		t.Errorf("Unexpected value for PC; expected %#02x, found %#02x", 0x2, state.PC())
	}
	// step the program for 1000 cycles, or until it hits the opcode 0x8b83
	// hitting 1000 cycles is considered failure
	for i := 0; i < 1000; i++ {
		if err := cycle(t, state); err != nil {
			t.Fatal(err)
		}
		if state.Ram.Load(state.PC()) == 0x8b83 { // sub PC, 1
			break
		}
	}
	if state.Ram.Load(state.PC()) != 0x8b83 {
		// we exhausted our steps
		logRegisters(t, state)
		t.Error("Program exceeded 1000 cycles")
	}
	// check 0x8000 - 0x800B for "Hello world!"
	expected := "Hello world!"
	for i := 0; i < len(expected); i++ {
		if state.Ram.Load(Word(0x8000+i)) != Word(expected[i]) {
			t.Errorf("Unexpected output in video ram; expected %v, found %v", []byte(expected), state.Ram.GetSlice(0x8000, 0x800B))
			break
		}
	}
}

var notchAssemblerTestProgram = [...]Word{
	//              set a, 0xbeef
	0x7c01, // 00
	0xbeef, // 01
	//              set [0x1000], a
	0x03c1, // 02
	0x1000, // 03
	//              ifn a, [0x1000]
	0x7813, // 04
	0x1000, // 05
	//                  set PC, end
	0x7f81, // 06
	0x001f, // 07
	//
	//              set i, 0
	0x84c1, // 08
	// :nextchar    ife [data+i], 0
	0x86d2, // 09
	0x0012, // 0a
	//                  set PC, end
	0x7f81, // 0b
	0x001f, // 0c
	//              set [0x8000+i], [data+i]
	0x5ac1, // 0d
	0x0012, // 0e
	0x8000, // 0f
	//              add i, 1
	0x88c2, // 10
	//              set PC, nextchar
	0xab81, // 11
	//
	// :data        dat "Hello world!", 0
	'H', 'e', 'l', 'l', 'o', ' ', 'w', 'o', 'r', 'l', 'd', '!', 0, // 12-1e
	//
	// :end         sub PC, 1
	0x8b83, // 1f
}

func TestNotchSpecExample(t *testing.T) {
	state := new(State)
	if err := state.LoadProgram(notchSpecExampleProgram[:], 0); err != nil {
		t.Fatal(err)
	}

	// test the first section
	for i := 0; i < 11; i++ {
		if err := cycle(t, state); err != nil {
			t.Fatal(err)
		}
	}
	if state.step != stateStepFetch {
		t.Fatalf("Unexpectedly stopped mid-instruction")
	}
	if state.A() != 0x10 {
		t.Errorf("Unexpected value for register A; expected %#x, found %#x", 0x10, state.A())
	}
	if state.PC() != 10 {
		t.Errorf("Unexpected value for register PC; expected %#x, found %#x", 10, state.A())
	}
	// run 21 more cycles (12 more instructions, partway into the loop)
	for i := 0; i < 21; i++ {
		if err := cycle(t, state); err != nil {
			t.Fatal(err)
		}
	}
	if state.step != stateStepFetch {
		t.Fatalf("Unexpectedly stopped mid-instruction")
	}
	if state.I() != 7 {
		t.Errorf("Unexpected value for register I; expected %#x, found %#x", 7, state.I())
	}
	if state.PC() != 16 {
		t.Errorf("Unexpected value for register PC; expected %#x, found %#x", 16, state.PC())
	}
	// 52 more cycles (29 more instructions) to finish the loop
	for i := 0; i < 52; i++ {
		if err := cycle(t, state); err != nil {
			t.Fatal(err)
		}
	}
	if state.step != stateStepFetch {
		t.Fatalf("Unexpectedly stopped mid-instruction")
	}
	if state.I() != 0 {
		t.Errorf("Unexpected value for register I; expected %#x, found %#x", 0, state.I())
	}
	if state.PC() != 0x12 {
		t.Errorf("Unexpected value for register PC; expected %#x, found %#x", 0x12, state.PC())
	}
	if state.SP() != 0 {
		t.Errorf("Unexpected value for register SP; expected %#x, found %#x", 0, state.SP())
	}
	// 5 more cycles (2 more instructions) to put us into the subroutine
	for i := 0; i < 5; i++ {
		if err := cycle(t, state); err != nil {
			t.Fatal(err)
		}
	}
	if state.step != stateStepFetch {
		t.Fatalf("Unexpectedly stopped mid-instruction")
	}
	if state.X() != 4 {
		t.Errorf("Unexpected value for register X; expected %#x, found %#x", 4, state.X())
	}
	if state.PC() != 0x17 {
		t.Errorf("Unexpected value for register PC; expected %#x, found %#x", 0x17, state.PC())
	}
	if state.SP() != 0xffff {
		t.Errorf("Unexpected value for register SP; expected %#x, found %#x", 0xffff, state.SP())
	}
	if state.Ram.Load(0xffff) != 0x15 {
		t.Errorf("Unexpected value at 0xffff; expected %#x, found %#x", 0x15, state.Ram.Load(0xffff))
		t.FailNow()
	}
	if t.Failed() {
		t.FailNow()
	}
	// run the program for 1000 cycles, or until it hits the instruction 0xeb81
	success := false
	for i := 0; i < 1000; i++ {
		if err := cycle(t, state); err != nil {
			t.Fatal(err)
		}
		if state.Ram.Load(state.PC()) == 0xeb81 {
			success = true
			break
		}
	}
	if !success {
		// we exhausted our steps
		logRegisters(t, state)
		t.Error("Program exceeded 1000 cycles")
	}

	// Check register X, it should be 0x40
	if state.X() != 0x40 {
		t.Error("Unexpected value for register X; expected %#x, found %#x", 0x40, state.X())
	}
}

var notchSpecExampleProgram = [...]Word{
	0x7c01, 0x0030, 0x7fc1, 0x0020, 0x1000, 0x7803, 0x1000, 0xc413,
	0x7f81, 0x0019, 0xacc1, 0x7c01, 0x2000, 0x22c1, 0x2000, 0x88c3,
	0x84d3, 0xbb81, 0x9461, 0x7c20, 0x0017, 0x7f81, 0x0019, 0x946e,
	0x6381, 0xeb81,
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
		if err := cycle(t, state); err != nil {
			t.Fatal(err)
		}
		if state.Ram.Load(state.PC()) == 0x85C3 { // sub PC, 1
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
