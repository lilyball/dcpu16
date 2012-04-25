package core

const (
	registerA = iota
	registerB
	registerC
	registerX
	registerY
	registerZ
	registerI
	registerJ
	registerSP
	registerPC
	registerEX
	registerIA
	registerCount
)

type Registers [registerCount]Word

func (r *Registers) A() Word {
	return r[registerA]
}

func (r *Registers) SetA(value Word) {
	r[registerA] = value
}

func (r *Registers) B() Word {
	return r[registerB]
}

func (r *Registers) SetB(value Word) {
	r[registerB] = value
}

func (r *Registers) C() Word {
	return r[registerC]
}

func (r *Registers) SetC(value Word) {
	r[registerC] = value
}

func (r *Registers) X() Word {
	return r[registerX]
}

func (r *Registers) SetX(value Word) {
	r[registerX] = value
}

func (r *Registers) Y() Word {
	return r[registerY]
}

func (r *Registers) SetY(value Word) {
	r[registerY] = value
}

func (r *Registers) Z() Word {
	return r[registerZ]
}

func (r *Registers) SetZ(value Word) {
	r[registerZ] = value
}

func (r *Registers) I() Word {
	return r[registerI]
}

func (r *Registers) SetI(value Word) {
	r[registerI] = value
}

func (r *Registers) J() Word {
	return r[registerJ]
}

func (r *Registers) SetJ(value Word) {
	r[registerJ] = value
}

func (r *Registers) SP() Word {
	return r[registerSP]
}

func (r *Registers) SetSP(value Word) {
	r[registerSP] = value
}

func (r *Registers) IncrSP() {
	r.SetSP(r.SP() + 1)
}

func (r *Registers) DecrSP() {
	r.SetSP(r.SP() - 1)
}

func (r *Registers) PC() Word {
	return r[registerPC]
}

func (r *Registers) SetPC(value Word) {
	r[registerPC] = value
}

func (r *Registers) IncrPC() {
	r.SetPC(r.PC() + 1)
}

func (r *Registers) EX() Word {
	return r[registerEX]
}

func (r *Registers) SetEX(value Word) {
	r[registerEX] = value
}

func (r *Registers) IA() Word {
	return r[registerIA]
}

func (r *Registers) SetIA(value Word) {
	r[registerIA] = value
}
