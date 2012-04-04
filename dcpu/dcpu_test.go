package dcpu

import (
	"testing"
)

func TestLoadProgram(t *testing.T) {
	state := new(State)
	if err := state.LoadProgram(notchAssemblerTestProgram[:], 0, true); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < len(notchAssemblerTestProgram); i++ {
		if state.Ram[i] != notchAssemblerTestProgram[i] {
			t.Errorf("Expected word %04x, found word %04x at offset %d", notchAssemblerTestProgram[i], state.Ram[i], i)
			break
		}
	}
}

func TestNotchAssemblerTest(t *testing.T) {
	state := new(State)
	if err := state.LoadProgram(notchAssemblerTestProgram[:], 0, true); err != nil {
		t.Fatal(err)
	}

	// step the program for 1000 steps, or until it hits the opcode 0x85C3
	// hitting 1000 steps is considered failure
	for i := 0; i < 1000; i++ {
		t.Logf("%d: %04x", state.PC, state.Ram[state.PC])
		if err := state.Step(); err != nil {
			t.Fatal(err)
			break
		}
		if state.Ram[state.PC] == 0x85C3 { // sub PC, 1
			break
		}
	}
	if state.Ram[state.PC] != 0x85C3 {
		// we exhausted our steps
		t.Error("Program exceeded 1000 steps")
	}
	// check 0x8000 - 0x800B for "Hello world!"
	expected := "Hello world!"
	for i := 0; i < len(expected); i++ {
		if state.Ram[0x8000+i] != Word(expected[i]) {
			t.Errorf("Unexpected output in video ram; expected %v, found %v", []byte(expected), state.Ram[0x8000:0x800B])
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
