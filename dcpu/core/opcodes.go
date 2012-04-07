package core

// basic opcodes
const (
	opcodeSET = 0x1
	opcodeADD = 0x2
	opcodeSUB = 0x3
	opcodeMUL = 0x4
	opcodeDIV = 0x5
	opcodeMOD = 0x6
	opcodeSHL = 0x7
	opcodeSHR = 0x8
	opcodeAND = 0x9
	opcodeBOR = 0xa
	opcodeXOR = 0xb
	opcodeIFE = 0xc
	opcodeIFN = 0xd
	opcodeIFG = 0xe
	opcodeIFB = 0xf
)

// non-basic opcodes
const (
	opcodeJSR = 0x1
)

// extended non-basic opcodes (internal representation)
const (
	opcodeExtJSR = 0x101
)
