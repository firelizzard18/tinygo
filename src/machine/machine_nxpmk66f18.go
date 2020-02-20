// +build nxp,mk66f18

package machine

import (
	"device/nxp"
	"errors"
	"runtime/interrupt"
	"runtime/volatile"

	"github.com/tinygo-org/tinygo/src/device/arm"
)

const (
	uartC2Enable       = nxp.UART_C2_TE | nxp.UART_C2_RE | nxp.UART_C2_RIE | nxp.UART_C2_ILIE
	uartC2TXActive     = uartC2Enable | nxp.UART_C2_TIE
	uartC2TXCompleting = uartC2Enable | nxp.UART_C2_TCIE
	uartC2TXInactive   = uartC2Enable

	uartIRQPriority = 64
)

var (
	ErrUnconfiguredUART = errors.New("unconfigured UART")
)

type FastPin struct {
	PDOR *volatile.BitRegister
	PSOR *volatile.BitRegister
	PCOR *volatile.BitRegister
	PTOR *volatile.BitRegister
	PDIR *volatile.BitRegister
	PDDR *volatile.BitRegister
}

type UARTConfig struct {
	BaudRate uint32
}

type pin struct {
	Bit  uint8
	PCR  *volatile.Register32
	GPIO *nxp.GPIO_Type
}

type uart struct {
	*nxp.UART_Type
	RXPCR    *volatile.Register32
	TXPCR    *volatile.Register32
	SCGC     *volatile.Register32
	SCGCMask uint32
}

var uartState = [5]struct {
	RXBuffer     RingBuffer
	TXBuffer     RingBuffer
	Interrupt    interrupt.Interrupt
	Transmitting volatile.Register8
}{}

// Configure this pin with the given configuration.
func (p Pin) Configure(config PinConfig) {
	switch config.Mode {
	case PinInput:
		panic("todo")

	case PinOutput:
		r := p.reg()
		r.GPIO.PDDR.SetBits(1 << r.Bit)
		r.PCR.SetBits(nxp.PORT_PCR0_SRE | nxp.PORT_PCR0_DSE | nxp.PORT_PCR0_MUX(1))
		r.PCR.ClearBits(nxp.PORT_PCR0_ODE)
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

// Configure the UART.
func (u UART) Configure(config UARTConfig) {
	if u == USBUART {
		return
	} else if u == LPUART {
		return
	}
	r := u.reg()
	s := uartState[u]

	// turn on the clock
	r.SCGC.Set(r.SCGCMask)

	// configure pins
	r.RXPCR.Set(nxp.PORT_PCR0_PE | nxp.PORT_PCR0_PS | nxp.PORT_PCR0_PFE | nxp.PORT_PCR0_MUX(3))
	r.TXPCR.Set(nxp.PORT_PCR0_DSE | nxp.PORT_PCR0_SRE | nxp.PORT_PCR0_MUX(3))

	// default to 115200 baud
	if config.BaudRate == 0 {
		config.BaudRate = 115200
	}

	// copied from teensy core's BAUD2DIV macro
	divisor := ((CPUFrequency() * 2) + ((config.BaudRate) >> 1)) / config.BaudRate

	r.BDH.Set(uint8((divisor >> 13) & 0x1F))
	r.BDL.Set(uint8((divisor >> 5) & 0xFF))
	r.C4.Set(uint8(divisor & 0x1F))
	r.C1.Set(nxp.UART_C1_ILT)
	r.TWFIFO.Set(2) // tx watermark, causes S1_TDRE to set
	r.RWFIFO.Set(4) // rx watermark, causes S1_RDRF to set
	r.PFIFO.Set(nxp.UART_PFIFO_TXFE | nxp.UART_PFIFO_RXFE)
	r.C2.Set(uartC2TXInactive)

	// if s.Interrupt == interrupt.Interrupt{} {
	s.Interrupt = u.newISR()
	s.Interrupt.SetPriority(uartIRQPriority)
	s.Interrupt.Enable()
	// }
}

// adapted from Teensy core's uart0_status_isr
func (u UART) handleStatusInterrupt() {
	r := u.reg()
	s := uartState[u]

	if r.S1.HasBits(nxp.UART_S1_RDRF | nxp.UART_S1_IDLE) {
		intrs := arm.DisableInterrupts()
		avail := r.RCFIFO.Get()
		if avail == 0 {
			// The only way to clear the IDLE interrupt flag is
			// to read the data register.  But reading with no
			// data causes a FIFO underrun, which causes the
			// FIFO to return corrupted data.  If anyone from
			// Freescale reads this, what a poor design!  There
			// write should be a write-1-to-clear for IDLE.
			r.D.Get()
			// flushing the fifo recovers from the underrun,
			// but there's a possible race condition where a
			// new character could be received between reading
			// RCFIFO == 0 and flushing the FIFO.  To minimize
			// the chance, interrupts are disabled so a higher
			// priority interrupt (hopefully) doesn't delay.
			// TODO: change this to disabling the IDLE interrupt
			// which won't be simple, since we already manage
			// which transmit interrupts are enabled.
			r.CFIFO.Set(nxp.UART_CFIFO_RXFLUSH)
			arm.EnableInterrupts(intrs)

		} else {
			arm.EnableInterrupts(intrs)

			for {
				s.RXBuffer.Put(r.D.Get())
				avail--
				if avail <= 0 {
					break
				}
			}
		}

		c := r.C2.Get()
		if c&nxp.UART_C2_TIE != 0 && r.S1.HasBits(nxp.UART_S1_TDRE) {
			for {
				n, ok := s.TXBuffer.Get()
				if !ok {
					break
				}

				r.S1.Get()
				r.D.Set(n)

				if r.TCFIFO.Get() >= 8 {
					break
				}
			}

			if r.S1.HasBits(nxp.UART_S1_TDRE) {
				r.C2.Set(uartC2TXCompleting)
			}
		}

		if c&nxp.UART_C2_TCIE != 0 && r.S1.HasBits(nxp.UART_S1_TC) {
			s.Transmitting.Set(0)
			r.C2.Set(uartC2TXInactive)
		}
	}
}

// WriteByte writes a byte of data to the UART.
func (u UART) WriteByte(c byte) error {
	if u == USBUART {
		return nil
	} else if u == LPUART {
		return nil
	}
	r := u.reg()
	s := uartState[u]

	// adapted from Teensy core's serial_putchar
	if !r.SCGC.HasBits(r.SCGCMask) {
		return ErrUnconfiguredUART
	}

	for {
		priority := arm.GetExecutionPriority()
		if priority < uartIRQPriority {
			if r.S1.HasBits(nxp.UART_S1_TDRE) {
				n, ok := s.TXBuffer.Get()
				if ok {
					r.D.Set(n)
				}
			}
		} else if priority >= 256 {
			// yield()
		}
	}

	s.TXBuffer.Put(c)
	s.Transmitting.Set(1)
	r.C2.Set(uartC2TXActive)
	return nil
}
