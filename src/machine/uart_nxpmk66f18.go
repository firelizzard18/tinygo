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
	"device/arm"
	"device/nxp"
	"errors"
	"runtime/interrupt"
	"runtime/volatile"

	_ "unsafe" // for go:linkname
)

const (
	uartC2Enable       = nxp.UART_C2_TE | nxp.UART_C2_RE | nxp.UART_C2_RIE | nxp.UART_C2_ILIE
	uartC2TXActive     = uartC2Enable | nxp.UART_C2_TIE
	uartC2TXCompleting = uartC2Enable | nxp.UART_C2_TCIE
	uartC2TXInactive   = uartC2Enable

	uartIRQPriority = 64

	// determined from UARTx_PFIFO
	uartRXFIFODepth = 8
	uartTXFIFODepth = 8
)

var (
	ErrNotImplemented = errors.New("device has not been implemented")
	ErrNotConfigured  = errors.New("device has not been configured")
)

//go:linkname gosched runtime.Gosched
func gosched()

type UART = *UARTData

type UARTData struct {
	*nxp.UART_Type
	SCGC      *volatile.Register32
	SCGCMask  uint32
	IRQNumber uint32

	DefaultRX Pin
	DefaultTX Pin

	// state
	Buffer       RingBuffer // RX Buffer
	TXBuffer     RingBuffer
	Configured   bool
	Transmitting volatile.Register8
	Interrupt    interrupt.Interrupt
}

// 'UART0' in the K66 manual corresponds to 'UART1' on the Teensy's pinout
var UART1 = UARTData{UART_Type: nxp.UART0, SCGC: &nxp.SIM.SCGC4, SCGCMask: nxp.SIM_SCGC4_UART0, IRQNumber: nxp.IRQ_UART0_RX_TX, DefaultRX: defaultUART1RX, DefaultTX: defaultUART1TX}
var UART2 = UARTData{UART_Type: nxp.UART1, SCGC: &nxp.SIM.SCGC4, SCGCMask: nxp.SIM_SCGC4_UART1, IRQNumber: nxp.IRQ_UART1_RX_TX, DefaultRX: defaultUART2RX, DefaultTX: defaultUART2TX}
var UART3 = UARTData{UART_Type: nxp.UART2, SCGC: &nxp.SIM.SCGC4, SCGCMask: nxp.SIM_SCGC4_UART2, IRQNumber: nxp.IRQ_UART2_RX_TX, DefaultRX: defaultUART3RX, DefaultTX: defaultUART3TX}
var UART4 = UARTData{UART_Type: nxp.UART3, SCGC: &nxp.SIM.SCGC4, SCGCMask: nxp.SIM_SCGC4_UART3, IRQNumber: nxp.IRQ_UART3_RX_TX, DefaultRX: defaultUART4RX, DefaultTX: defaultUART4TX}
var UART5 = UARTData{UART_Type: nxp.UART4, SCGC: &nxp.SIM.SCGC1, SCGCMask: nxp.SIM_SCGC1_UART4, IRQNumber: nxp.IRQ_UART4_RX_TX, DefaultRX: defaultUART5RX, DefaultTX: defaultUART5TX}

func init() {
	UART1.Interrupt = interrupt.New(nxp.IRQ_UART0_RX_TX, UART1.handleStatusInterrupt)
	UART2.Interrupt = interrupt.New(nxp.IRQ_UART1_RX_TX, UART2.handleStatusInterrupt)
	UART3.Interrupt = interrupt.New(nxp.IRQ_UART2_RX_TX, UART3.handleStatusInterrupt)
	UART4.Interrupt = interrupt.New(nxp.IRQ_UART3_RX_TX, UART4.handleStatusInterrupt)
	UART5.Interrupt = interrupt.New(nxp.IRQ_UART4_RX_TX, UART5.handleStatusInterrupt)
}

// Configure the UART.
func (u UART) Configure(config UARTConfig) {
	// from: serial_begin

	if !u.Configured {
		u.Transmitting.Set(0)

		// turn on the clock
		u.SCGC.Set(u.SCGCMask)

		// configure pins
		u.DefaultRX.Control().Set(nxp.PORT_PCR0_PE | nxp.PORT_PCR0_PS | nxp.PORT_PCR0_PFE | nxp.PORT_PCR0_MUX(3))
		u.DefaultTX.Control().Set(nxp.PORT_PCR0_DSE | nxp.PORT_PCR0_SRE | nxp.PORT_PCR0_MUX(3))
		u.C1.Set(nxp.UART_C1_ILT)
	}

	// default to 115200 baud
	if config.BaudRate == 0 {
		config.BaudRate = 115200
	}

	// copied from teensy core's BAUD2DIV macro
	divisor := ((CPUFrequency() * 2) + (config.BaudRate >> 1)) / config.BaudRate
	if divisor < 32 {
		divisor = 32
	}

	if u.Configured {
		// don't change baud rate mid transmit
		for u.Transmitting.Get() != 0 {
			// busy wait flush, for compatibility with putchar
		}
	}

	// set the divisor
	u.BDH.Set(uint8((divisor >> 13) & 0x1F))
	u.BDL.Set(uint8((divisor >> 5) & 0xFF))
	u.C4.Set(uint8(divisor & 0x1F))

	if !u.Configured {
		u.Configured = true

		u.C1.Set(nxp.UART_C1_ILT)

		// configure TX and RX watermark
		u.TWFIFO.Set(2) // causes bit TDRE of S1 to set
		u.RWFIFO.Set(4) // causes bit RDRF of S1 to set

		// enable FIFOs
		u.PFIFO.Set(nxp.UART_PFIFO_TXFE | nxp.UART_PFIFO_RXFE)

		// setup interrupts
		u.C2.Set(uartC2TXInactive)
		u.Interrupt.SetPriority(uartIRQPriority)
		u.Interrupt.Enable()
	}
}

func (u UART) Disable() {
	// from: serial_end

	// check if the device has been enabled already
	if !u.SCGC.HasBits(u.SCGCMask) {
		return
	}

	u.Flush()

	u.Interrupt.Disable()
	u.C2.Set(0)

	// reconfigure pin
	u.DefaultRX.Configure(PinConfig{Mode: PinInputPullUp})
	u.DefaultTX.Configure(PinConfig{Mode: PinInputPullUp})

	// clear flags
	u.S1.Get()
	u.D.Get()
	u.Buffer.Clear()
}

func (u UART) Flush() {
	for u.Transmitting.Get() != 0 {
		gosched()
	}
}

func (u UART) handleStatusInterrupt(interrupt.Interrupt) {
	// from: uart0_status_isr

	// receive
	if u.S1.HasBits(nxp.UART_S1_RDRF | nxp.UART_S1_IDLE) {
		intrs := arm.DisableInterrupts()
		avail := u.RCFIFO.Get()
		if avail == 0 {
			// The only way to clear the IDLE interrupt flag is
			// to read the data register.  But reading with no
			// data causes a FIFO underrun, which causes the
			// FIFO to return corrupted data.  If anyone from
			// Freescale reads this, what a poor design!  There
			// write should be a write-1-to-clear for IDLE.
			u.D.Get()
			// flushing the fifo recovers from the underrun,
			// but there's a possible race condition where a
			// new character could be received between reading
			// RCFIFO == 0 and flushing the FIFO.  To minimize
			// the chance, interrupts are disabled so a higher
			// priority interrupt (hopefully) doesn't delay.
			// TODO: change this to disabling the IDLE interrupt
			// which won't be simple, since we already manage
			// which transmit interrupts are enabled.
			u.CFIFO.Set(nxp.UART_CFIFO_RXFLUSH)
			arm.EnableInterrupts(intrs)

		} else {
			arm.EnableInterrupts(intrs)

			for {
				u.Buffer.Put(u.D.Get())
				avail--
				if avail <= 0 {
					break
				}
			}
		}
	}

	// transmit
	if u.C2.HasBits(nxp.UART_C2_TIE) && u.S1.HasBits(nxp.UART_S1_TDRE) {
		data := make([]byte, 0, uartTXFIFODepth)
		avail := uartTXFIFODepth - u.TCFIFO.Get()

		// get avail bytes from ring buffer
		for len(data) < int(avail) {
			if b, ok := u.TXBuffer.Get(); ok {
				data = append(data, b)
			} else {
				break
			}
		}

		// write data to FIFO
		l := len(data)
		for i, b := range data {
			if i == l-1 {
				// only clear TDRE on last write, per the manual
				u.S1.Get()
			}
			u.D.Set(b)
		}

		// if FIFO still has room, disable TIE, enable TCIE
		if u.S1.HasBits(nxp.UART_S1_TDRE) {
			u.C2.Set(uartC2TXCompleting)
		}
	}

	// transmit complete
	if u.C2.HasBits(nxp.UART_C2_TCIE) && u.S1.HasBits(nxp.UART_S1_TC) {
		u.Transmitting.Set(0)
		u.C2.Set(uartC2TXInactive)
	}
}

// WriteByte writes a byte of data to the UART.
func (u UART) WriteByte(c byte) error {
	if !u.Configured {
		return ErrNotConfigured
	}

	for !u.TXBuffer.Put(c) {
		gosched()
	}

	u.Transmitting.Set(1)
	u.C2.Set(uartC2TXActive)
	return nil
}
