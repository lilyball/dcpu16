package core

type Opcode uint16

// basic opcodes
const (
	opcodeSET Opcode = 0x01
	opcodeADD        = 0x02
	opcodeSUB        = 0x03
	opcodeMUL        = 0x04
	opcodeMLI        = 0x05
	opcodeDIV        = 0x06
	opcodeDVI        = 0x07
	opcodeMOD        = 0x08
	opcodeAND        = 0x09
	opcodeBOR        = 0x0a
	opcodeXOR        = 0x0b
	opcodeSHR        = 0x0c
	opcodeASR        = 0x0d
	opcodeSHL        = 0x0e
	/* 0x0f is reserved */
	opcodeIFB = 0x10
	opcodeIFC = 0x11
	opcodeIFE = 0x12
	opcodeIFN = 0x13
	opcodeIFG = 0x14
	opcodeIFA = 0x15
	opcodeIFL = 0x16
	opcodeIFU = 0x17
)

// non-basic opcodes
const (
	opcodeJSR Opcode = 0x1
	/* 0x02-0x07 are reserved */
	opcodeINT = 0x08
	opcodeIAG = 0x09 // spec says ING, but it's probably a typo
	opcodeIAS = 0x0a // spec says INS, but it's probably a typo
	/* 0x0b-0x0f are reserved */
	opcodeHWN = 0x10
	opcodeHWQ = 0x11
	opcodeHWI = 0x12
)

// extended non-basic opcodes (internal representation)
const (
	opcodeExtJSR Opcode = opcodeJSR | opcodeExtendedOffset
	opcodeExtINT        = opcodeINT | opcodeExtendedOffset
	opcodeExtIAG        = opcodeIAG | opcodeExtendedOffset
	opcodeExtIAS        = opcodeIAS | opcodeExtendedOffset
	opcodeExtHWN        = opcodeHWN | opcodeExtendedOffset
	opcodeExtHWQ        = opcodeHWQ | opcodeExtendedOffset
	opcodeExtHWI        = opcodeHWI | opcodeExtendedOffset
)
const opcodeExtendedOffset = 0x100
