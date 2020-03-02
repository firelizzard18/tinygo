// Derivative work of Teensyduino Core Library
// http://www.pjrc.com/teensy/
// Copyright (c) 2017 PJRC.COM, LLC.
//
// Permission is hereby granted, free of charge, to any person obtaining
// a copy of this software and associated documentation files (the
// "Software"), to deal in the Software without restriction, including
// without limitation the rights to use, copy, modify, merge, publish,
// distribute, sublicense, and/or sell copies of the Software, and to
// permit persons to whom the Software is furnished to do so, subject to
// the following conditions:
//
// 1. The above copyright notice and this permission notice shall be
// included in all copies or substantial portions of the Software.
//
// 2. If the Software is incorporated into a build system that allows
// selection among a list of target devices, then similar target
// devices manufactured by PJRC.COM must be included in the list of
// target devices and selectable in the same manner.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
// EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
// MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
// NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS
// BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN
// ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
// CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

// +build nxp,mk66f18

package machine

import (
	"device/nxp"
	"runtime/volatile"
	"unsafe"
)

//go:inline
func (p Pin) reg() (*nxp.GPIO_Type, *volatile.Register32, uint8) {
	var gpio *nxp.GPIO_Type
	var pcr *nxp.PORT_Type

	if p < 32 {
		gpio, pcr = nxp.GPIOA, nxp.PORTA
	} else if p < 64 {
		gpio, pcr = nxp.GPIOB, nxp.PORTB
	} else if p < 96 {
		gpio, pcr = nxp.GPIOC, nxp.PORTC
	} else if p < 128 {
		gpio, pcr = nxp.GPIOD, nxp.PORTD
	} else if p < 160 {
		gpio, pcr = nxp.GPIOE, nxp.PORTE
	} else {
		panic("invalid pin number")
	}

	return gpio, &(*[32]volatile.Register32)(unsafe.Pointer(pcr))[p%32], uint8(p % 32)
}

func (p Pin) configure(mode PinMode, alt uint8) {
	gpio, pcr, pos := p.reg()
	mux := uint32(alt)

	switch mode {
	case PinOutput:
		gpio.PDDR.SetBits(1 << pos)
		pcr.Set(nxp.PORT_PCR0_MUX(1) | nxp.PORT_PCR0_SRE | nxp.PORT_PCR0_DSE)

	case PinOutputOpenDrain:
		gpio.PDDR.SetBits(1 << pos)
		pcr.Set(nxp.PORT_PCR0_MUX(1) | nxp.PORT_PCR0_SRE | nxp.PORT_PCR0_DSE | nxp.PORT_PCR0_ODE)

	case PinInput:
		gpio.PDDR.ClearBits(1 << pos)
		pcr.Set(nxp.PORT_PCR0_MUX(1))

	case PinInputPullUp:
		gpio.PDDR.ClearBits(1 << pos)
		pcr.Set(nxp.PORT_PCR0_MUX(1) | nxp.PORT_PCR0_PE | nxp.PORT_PCR0_PS)

	case PinInputPullDown:
		gpio.PDDR.ClearBits(1 << pos)
		pcr.Set(nxp.PORT_PCR0_MUX(1) | nxp.PORT_PCR0_PE)

	case PinDisable:
		gpio.PDDR.ClearBits(1 << pos)
		pcr.Set(nxp.PORT_PCR0_MUX(0))

	case uartPinRX:
		pcr.Set(nxp.PORT_PCR0_PE | nxp.PORT_PCR0_PS | nxp.PORT_PCR0_PFE | nxp.PORT_PCR0_MUX(mux))

	case uartPinTX:
		pcr.Set(nxp.PORT_PCR0_DSE | nxp.PORT_PCR0_SRE | nxp.PORT_PCR0_MUX(mux))
	}
}

// Configure this pin with the given configuration.
func (p Pin) Configure(config PinConfig) {
	p.configure(config.Mode, 1)
}

// Set changes the value of the GPIO pin. The pin must be configured as output.
func (p Pin) Set(value bool) {
	gpio, _, pos := p.reg()
	if value {
		gpio.PSOR.Set(1 << pos)
	} else {
		gpio.PCOR.Set(1 << pos)
	}
}

// Get returns the current value of a GPIO pin.
func (p Pin) Get() bool {
	gpio, _, pos := p.reg()
	return gpio.PDIR.HasBits(1 << pos)
}

func (p Pin) Control() *volatile.Register32 {
	_, pcr, _ := p.reg()
	return pcr
}

func (p Pin) Fast() FastPin {
	gpio, _, pos := p.reg()
	return FastPin{
		PDOR: gpio.PDOR.Bit(pos),
		PSOR: gpio.PSOR.Bit(pos),
		PCOR: gpio.PCOR.Bit(pos),
		PTOR: gpio.PTOR.Bit(pos),
		PDIR: gpio.PDIR.Bit(pos),
		PDDR: gpio.PDDR.Bit(pos),
	}
}

type FastPin struct {
	PDOR *volatile.BitRegister
	PSOR *volatile.BitRegister
	PCOR *volatile.BitRegister
	PTOR *volatile.BitRegister
	PDIR *volatile.BitRegister
	PDDR *volatile.BitRegister
}

func (p FastPin) Set()         { p.PSOR.Set(true) }
func (p FastPin) Clear()       { p.PCOR.Set(true) }
func (p FastPin) Toggle()      { p.PTOR.Set(true) }
func (p FastPin) Write(v bool) { p.PDOR.Set(v) }
func (p FastPin) Read() bool   { return p.PDIR.Get() }
