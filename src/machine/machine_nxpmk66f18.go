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
)

type PinMode uint8

const (
	PinInput PinMode = iota
	PinInputPullUp
	PinInputPullDown
	PinOutput
	PinOutputOpenDrain
	PinDisable
)

type FastPin struct {
	PDOR *volatile.BitRegister
	PSOR *volatile.BitRegister
	PCOR *volatile.BitRegister
	PTOR *volatile.BitRegister
	PDIR *volatile.BitRegister
	PDDR *volatile.BitRegister
}

type pin struct {
	Bit  uint8
	PCR  *volatile.Register32
	GPIO *nxp.GPIO_Type
}

// Configure this pin with the given configuration.
func (p Pin) Configure(config PinConfig) {
	r := p.reg()

	switch config.Mode {
	case PinOutput:
		r.GPIO.PDDR.SetBits(1 << r.Bit)
		r.PCR.Set(nxp.PORT_PCR0_MUX(1) | nxp.PORT_PCR0_SRE | nxp.PORT_PCR0_DSE)

	case PinOutputOpenDrain:
		r.GPIO.PDDR.SetBits(1 << r.Bit)
		r.PCR.Set(nxp.PORT_PCR0_MUX(1) | nxp.PORT_PCR0_SRE | nxp.PORT_PCR0_DSE | nxp.PORT_PCR0_ODE)

	case PinInput:
		r.GPIO.PDDR.ClearBits(1 << r.Bit)
		r.PCR.Set(nxp.PORT_PCR0_MUX(1))

	case PinInputPullUp:
		r.GPIO.PDDR.ClearBits(1 << r.Bit)
		r.PCR.Set(nxp.PORT_PCR0_MUX(1) | nxp.PORT_PCR0_PE | nxp.PORT_PCR0_PS)

	case PinInputPullDown:
		r.GPIO.PDDR.ClearBits(1 << r.Bit)
		r.PCR.Set(nxp.PORT_PCR0_MUX(1) | nxp.PORT_PCR0_PE)

	case PinDisable:
		r.GPIO.PDDR.ClearBits(1 << r.Bit)
		r.PCR.Set(nxp.PORT_PCR0_MUX(0))
	}
}

// Set changes the value of the GPIO pin. The pin must be configured as output.
func (p Pin) Set(value bool) {
	r := p.reg()
	if value {
		r.GPIO.PSOR.Set(1 << r.Bit)
	} else {
		r.GPIO.PCOR.Set(1 << r.Bit)
	}
}

// Get returns the current value of a GPIO pin.
func (p Pin) Get() bool {
	r := p.reg()
	return r.GPIO.PDIR.HasBits(1 << r.Bit)
}

func (p Pin) Control() *volatile.Register32 {
	return p.reg().PCR
}

func (p Pin) Fast() FastPin {
	r := p.reg()
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
