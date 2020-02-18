// +build nxp,mk66f18

package machine

import (
	"device/nxp"
	"runtime/volatile"
)

const (
	PortControlRegisterSRE = nxp.PORT_PCR0_SRE
	PortControlRegisterDSE = nxp.PORT_PCR0_DSE
	PortControlRegisterODE = nxp.PORT_PCR0_ODE
)

func PortControlRegisterMUX(v uint8) uint32 {
	return nxp.PORT_PCR0_MUX(uint32(v))
}

type pinMapping struct {
	Bit  uint8
	PCR  *volatile.Register32
	GPIO *nxp.GPIO_Type
}

// Configure this pin with the given configuration.
func (p Pin) Configure(config PinConfig) {
	switch config.Mode {
	case PinInput:
		panic("todo")

	case PinOutput:
		r := p.registers()
		r.GPIO.PDDR.SetBits(1 << r.Bit)
		r.PCR.SetBits(PortControlRegisterSRE | PortControlRegisterDSE | PortControlRegisterMUX(1))
		r.PCR.ClearBits(PortControlRegisterODE)
	}
}

// Set changes the value of the GPIO pin. The pin must be configured as output.
func (p Pin) Set(value bool) {
	r := p.registers()
	if value {
		r.GPIO.PSOR.Set(1 << r.Bit)
	} else {
		r.GPIO.PCOR.Set(1 << r.Bit)
	}
}

// Get returns the current value of a GPIO pin.
func (p Pin) Get() bool {
	r := p.registers()
	return r.GPIO.PDIR.HasBits(1 << r.Bit)
}

func (p Pin) Control() *volatile.Register32 {
	return p.registers().PCR
}

type FastPin struct {
	PDOR *volatile.BitRegister
	PSOR *volatile.BitRegister
	PCOR *volatile.BitRegister
	PTOR *volatile.BitRegister
	PDIR *volatile.BitRegister
	PDDR *volatile.BitRegister
}

func (p Pin) Fast() FastPin {
	r := p.registers()
	return FastPin{
		PDOR: r.GPIO.PDOR.Bit(r.Bit),
		PSOR: r.GPIO.PSOR.Bit(r.Bit),
		PCOR: r.GPIO.PCOR.Bit(r.Bit),
		PTOR: r.GPIO.PTOR.Bit(r.Bit),
		PDIR: r.GPIO.PDIR.Bit(r.Bit),
		PDDR: r.GPIO.PDDR.Bit(r.Bit),
	}
}

func (p FastPin) Set()         { p.PSOR.Set(true) }
func (p FastPin) Clear()       { p.PCOR.Set(true) }
func (p FastPin) Toggle()      { p.PTOR.Set(true) }
func (p FastPin) Write(v bool) { p.PDOR.Set(v) }
func (p FastPin) Read() bool   { return p.PDIR.Get() }
