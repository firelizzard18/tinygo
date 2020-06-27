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
	"fmt"
	"reflect"
	"runtime/interrupt"
	"runtime/volatile"
	"unsafe"
)

const (
	usb_BDT_Own   = 0x80
	usb_BDT_Data1 = 0x40
	usb_BDT_Keep  = 0x20
	nsb_BDT_NoInc = 0x10
	usb_BDT_DTS   = 0x08
	usb_BDT_Stall = 0x04

	usb_BDT_TOK_PID_Mask = 0b00111100
	usb_BDT_TOK_PID_Pos  = 2

	usb_TOK_PID_OUT   = 0x1
	usb_TOK_PID_IN    = 0x9
	usb_TOK_PID_SETUP = 0xD
)

//go:linkname millisSinceBoot runtime.millisSinceBoot
func millisSinceBoot() uint64

type usbBufferDescriptor struct {
	descriptor volatile.Register32
	address    volatile.Register32
}

type usbBufferDescriptorTable_t struct {
	records []usbBufferDescriptor
	retain  [][]byte
}

type usbBufferDescriptorEntry struct {
	record *usbBufferDescriptor
	retain *[]byte
}

type usbBufferDescriptorOwner bool

const usbBufferOwnedByUSBFS usbBufferDescriptorOwner = true
const usbBufferOwnedByProcessor usbBufferDescriptorOwner = false

func (bdt usbBufferDescriptorTable_t) Entry(i int) usbBufferDescriptorEntry {
	return usbBufferDescriptorEntry{&bdt.records[i], &bdt.retain[i]}
}

func (bdt usbBufferDescriptorTable_t) Get(ep uint16, tx, odd bool) usbBufferDescriptorEntry {
	i := ep << 2
	if tx {
		i |= 2
	}
	if odd {
		i |= 1
	}
	return bdt.Entry(int(i))
}

func (bdt usbBufferDescriptorTable_t) GetStat(stat uint8) usbBufferDescriptorEntry {
	return bdt.Entry(int(stat >> 2))
}

func (bde usbBufferDescriptorEntry) Reset() {
	bde.record.descriptor.Set(0)
	bde.record.address.Set(0)
	*bde.retain = nil
}

func (bde usbBufferDescriptorEntry) Describe(size uint32, data1 bool) {
	if size >= 1024 {
		panic("USB Buffer Descriptor overflow")
	}

	d := size<<16 | usb_BDT_Own | usb_BDT_DTS
	if data1 {
		d |= usb_BDT_Data1
	}
	bde.record.descriptor.Set(d)
}

func (bde usbBufferDescriptorEntry) Set(data []byte, data1 bool) {
	if len(data) == 0 {
		// send an empty response
		*bde.retain = nil
		bde.record.address.Set(0)
		bde.Describe(0, data1)
		return
	}

	if len(data) >= 1024 {
		panic("USB Buffer Descriptor overflow")
	}

	slice := (*reflect.SliceHeader)(unsafe.Pointer(&data))

	*bde.retain = data
	bde.record.address.Set(uint32(slice.Data))
	bde.Describe(uint32(slice.Len), data1)
}

func (bde usbBufferDescriptorEntry) Data() []byte {
	length := bde.record.descriptor.Get() >> 16

	var data []byte
	slice := (*reflect.SliceHeader)(unsafe.Pointer(&data))
	slice.Len = uintptr(length)
	slice.Cap = uintptr(length)
	slice.Data = uintptr(bde.record.address.Get())
	return data
}

func (bde usbBufferDescriptorEntry) TokenPID() uint32 {
	return (bde.record.descriptor.Get() & usb_BDT_TOK_PID_Mask) >> usb_BDT_TOK_PID_Pos
}

func (bde usbBufferDescriptorEntry) Owner() usbBufferDescriptorOwner {
	if bde.record.descriptor.HasBits(usb_BDT_Own) {
		return usbBufferOwnedByUSBFS
	}
	return usbBufferOwnedByProcessor
}

//go:align 512
var usbBufferDescriptorRecords [(usb_EndpointCount + 1) * 4]usbBufferDescriptor

var usbBufferDescriptorTable = usbBufferDescriptorTable_t{usbBufferDescriptorRecords[:], make([][]byte, len(usbBufferDescriptorRecords))}

var USB0 = USB{USB0_Type: nxp.USB0, SCGC: &nxp.SIM.SCGC4, SCGCMask: nxp.SIM_SCGC4_USBOTG}

func init() {
	// USB0.Interrupt = interrupt.New(nxp.IRQ_USB0, USB0.mainISR)
	// USB0.CDC = &USBCDC{&USB0, NewRingBuffer()}
}

func setupUSB() {
	// from: usb_init

	for i := range usbBufferDescriptorRecords {
		usbBufferDescriptorTable.Entry(i).Reset()
	}

	// this basically follows the flowchart in the Kinetis
	// Quick Reference User Guide, Rev. 1, 03/2012, page 141

	// assume 48 MHz clock already running
	// SIM - enable clock
	nxp.SIM.SCGC4.SetBits(nxp.SIM_SCGC4_USBOTG)
	nxp.SYSMPU.RGDAAC0.SetBits(0x03000000)

	// if using IRC48M, turn on the USB clock recovery hardware
	nxp.USB0.CLK_RECOVER_IRC_EN.Set(nxp.USB0_CLK_RECOVER_IRC_EN_IRC_EN | nxp.USB0_CLK_RECOVER_IRC_EN_REG_EN)
	nxp.USB0.CLK_RECOVER_CTRL.Set(nxp.USB0_CLK_RECOVER_CTRL_CLOCK_RECOVER_EN | nxp.USB0_CLK_RECOVER_CTRL_RESTART_IFRTRIM_EN)

	// // reset USB module
	// nxp.USB0.USBTRC0.Set(nxp.USB0_USBTRC_USBRESET)
	// for nxp.USB0.USBTRC0.HasBits(nxp.USB0_USBTRC_USBRESET) {} // wait for reset to end

	// set desc table base addr
	table := uintptr(unsafe.Pointer(&usbBufferDescriptorRecords[0]))
	if table&0x1FF != 0 {
		panic("USB Buffer Descriptor Table is not 512-byte aligned")
	}
	nxp.USB0.BDTPAGE1.Set(uint8(table >> 8))
	nxp.USB0.BDTPAGE2.Set(uint8(table >> 16))
	nxp.USB0.BDTPAGE3.Set(uint8(table >> 24))

	// clear all ISR flags
	nxp.USB0.ISTAT.Set(0xFF)
	nxp.USB0.ERRSTAT.Set(0xFF)
	nxp.USB0.OTGISTAT.Set(0xFF)

	//nxp.USB0.USBTRC0.SetBits(0x40) // undocumented bit

	// enable USB
	nxp.USB0.CTL.Set(nxp.USB0_CTL_USBENSOFEN)
	nxp.USB0.USBCTRL.Set(0)

	// enable reset interrupt
	nxp.USB0.INTEN.Set(nxp.USB0_INTEN_USBRSTEN)

	USB0.Interrupt = interrupt.New(nxp.IRQ_USB0, USB0.mainISR)
	USB0.Interrupt.SetPriority(112)
	USB0.Interrupt.Enable()

	// enable d+ pullup
	nxp.USB0.CONTROL.Set(nxp.USB0_CONTROL_DPPULLUPNONOTG)
}

// PollUSB manually checks a USB status and calls the ISR. This should only be
// called by runtime.abort.
func PollUSB(u *USB) {
	if u.SCGC.HasBits(u.SCGCMask) {
		u.mainISR(u.Interrupt)
	}
}

type USBCDC struct {
	Buffer *RingBuffer
}

func (u USBCDC) WriteByte(byte) {
	panic("not implemented")
}

var (
	//go:align 4
	ep0_rx0_buf [usb_BUFFER_SIZE]byte

	//go:align 4
	ep0_rx1_buf [usb_BUFFER_SIZE]byte
)

type USB struct {
	*nxp.USB0_Type
	SCGC     *volatile.Register32
	SCGCMask uint32

	// state
	Interrupt interrupt.Interrupt

	DataReceiveBuffer RingBuffer

	usb_configuration            volatile.Register8
	usb_reboot_timer             volatile.Register8
	usb_cdc_transmit_flush_timer volatile.Register8
}

func (u *USB) CDC() USBCDC {
	return USBCDC{&u.DataReceiveBuffer}
}

var USBMessages []string

func (u *USB) mainISR(interrupt.Interrupt) {
	// from: usb_isr

	// ARM errata 838869: Store immediate overlapping exception return operation
	// might vector to incorrect interrupt
	defer func() { arm.Asm("dsb 0xF") }()

again:
	status := u.ISTAT.Get() & u.INTEN.Get()

	if status&^nxp.USB0_ISTAT_SOFTOK != 0 {
		// log interrupt status, but not start of frame token
		USBMessages = append(USBMessages, fmt.Sprintf("istat %02x", status))
	}

	if status&nxp.USB0_ISTAT_SOFTOK != 0 {
		if u.usb_configuration.Get() != 0 {
			t := u.usb_reboot_timer.Get()
			if t != 0 {
				t--
				u.usb_reboot_timer.Set(t)
				if t == 0 {
					EnterBootloader()
				}
			}

			t = u.usb_cdc_transmit_flush_timer.Get()
			if t != 0 {
				t--
				u.usb_cdc_transmit_flush_timer.Set(t)
				if t == 0 {
					// usb_serial_flush_callback();
				}
			}
		}
		u.ISTAT.Set(nxp.USB0_ISTAT_SOFTOK)
	}

	if status&nxp.USB0_ISTAT_TOKDNE != 0 {
		stat := u.STAT.Get()
		ep := stat >> 4
		USBMessages = append(USBMessages, fmt.Sprintf("stat %02x", stat))
		if ep == 0 {
			u.handleControl(stat)
		} else {
			// endpoint is index to zero-based arrays
			u.handleEndpoint(ep-1, stat)
		}

		u.ISTAT.Set(nxp.USB0_ISTAT_TOKDNE)
		goto again
	}

	if status&nxp.USB0_ISTAT_USBRST != 0 {
		// initialize BDT toggle bits
		u.CTL.Set(nxp.USB0_CTL_ODDRST)
		ep0_tx_bdt_bank = false

		// set up buffers to receive Setup and OUT packets
		usbBufferDescriptorTable.Get(0, false, false).Set(ep0_rx0_buf[:], false) // EP0 RX even
		usbBufferDescriptorTable.Get(0, false, true).Set(ep0_rx1_buf[:], false)  // EP0 RX odd
		usbBufferDescriptorTable.Get(0, true, false).Reset()                     // EP0 TX even
		usbBufferDescriptorTable.Get(0, true, true).Reset()                      // EP0 TX odd

		// activate endpoint 0
		u.ENDPT0.Set(nxp.USB0_ENDPT_EPRXEN | nxp.USB0_ENDPT_EPTXEN | nxp.USB0_ENDPT_EPHSHK)

		// clear all ending interrupts
		u.ERRSTAT.Set(0xFF)
		u.ISTAT.Set(0xFF)

		// set the address to zero during enumeration
		u.ADDR.Set(0)

		// enable other interrupts
		u.ERREN.Set(0xFF)
		u.INTEN.Set(nxp.USB0_INTEN_TOKDNEEN |
			nxp.USB0_INTEN_SOFTOKEN |
			nxp.USB0_INTEN_STALLEN |
			nxp.USB0_INTEN_ERROREN |
			nxp.USB0_INTEN_USBRSTEN |
			nxp.USB0_INTEN_SLEEPEN)

		// is this necessary?
		u.CTL.Set(nxp.USB0_CTL_USBENSOFEN)
		return
	}

	if status&nxp.USB0_ISTAT_STALL != 0 {
		u.ENDPT0.Set(nxp.USB0_ENDPT_EPRXEN | nxp.USB0_ENDPT_EPTXEN | nxp.USB0_ENDPT_EPHSHK)
		u.ISTAT.Set(nxp.USB0_ISTAT_STALL)
	}

	if status&nxp.USB0_ISTAT_ERROR != 0 {
		err := u.ERRSTAT.Get()
		u.ERRSTAT.Set(err)
		USBMessages = append(USBMessages, fmt.Sprintf("errstat %02x", err))
		// TODO log error?
		u.ISTAT.Set(nxp.USB0_ISTAT_ERROR)
	}

	if status&nxp.USB0_ISTAT_SLEEP != 0 {
		u.ISTAT.Set(nxp.USB0_ISTAT_SLEEP)
	}
}

var setup usbSetup

func (u *USB) handleControl(stat uint8) {
	// from: usb_control

	descriptor := usbBufferDescriptorTable.GetStat(stat)

	pid := descriptor.TokenPID()
	USBMessages = append(USBMessages, fmt.Sprintf("bd tok 0x%X", pid))

	switch pid {
	case usb_TOK_PID_OUT, 0x2: // OUT transaction received from host
		// give the buffer back
		descriptor.Describe(usb_BUFFER_SIZE, true)

	case usb_TOK_PID_IN: // IN transaction completed to host
		data := ep0_tx_remainder
		if len(data) > 0 {
			ep0_tx_remainder = ep0_transmit_chunk(data)
		}

		if setup.bRequest == usb_SET_ADDRESS && setup.bmRequestType == 0 {
			setup.bRequest = 0
			u.ADDR.Set(setup.wValueL)
		}

	case usb_TOK_PID_SETUP:
		// read setup info
		data := descriptor.Data()
		setup = newUSBSetup(data)
		USBMessages = append(USBMessages, fmt.Sprintf("setup raw %x", data))

		// give the buffer back
		descriptor.Describe(usb_BUFFER_SIZE, true)

		// clear any leftover pending IN transactions
		ep0_tx_remainder = nil
		usbBufferDescriptorTable.Get(0, true, false).Reset() // EP0 TX even
		usbBufferDescriptorTable.Get(0, true, true).Reset()  // EP0 TX odd

		// first IN after Setup is always DATA1
		ep0_tx_data_toggle = true

		u.setup()
	}

	// unfreeze the USB, now that we're ready (clear TXSUSPENDTOKENBUSY bit)
	u.CTL.Set(nxp.USB0_CTL_USBENSOFEN)
}

func (u *USB) handleEndpoint(ep, stat uint8) {
	// TODO support other endpoints

	// CDC Data
	if stat&0x08 != 0 {
		u.handleDataTransmit(ep, stat)
	} else {
		u.handleDataReceive(ep, stat)
	}
}

func (u *USB) handleDataTransmit(ep, stat uint8) {
	panic("not supported")
}

func (u *USB) handleDataReceive(ep, stat uint8) {
	descriptor := usbBufferDescriptorTable.GetStat(stat)

	for _, c := range descriptor.Data() {
		u.DataReceiveBuffer.Put(c)
	}

	odd := stat&nxp.USB0_STAT_ODD != 0 // why?
	descriptor.Describe(usb_BUFFER_SIZE, odd)
}

func (u *USB) Endpoint(i uint16) usbEndpoint {
	switch i {
	case 0:
		return usbEndpoint{&u.ENDPT0}
	case 1:
		return usbEndpoint{&u.ENDPT1}
	case 2:
		return usbEndpoint{&u.ENDPT2}
	case 3:
		return usbEndpoint{&u.ENDPT3}
	case 4:
		return usbEndpoint{&u.ENDPT4}
	case 5:
		return usbEndpoint{&u.ENDPT5}
	case 6:
		return usbEndpoint{&u.ENDPT6}
	case 7:
		return usbEndpoint{&u.ENDPT7}
	case 8:
		return usbEndpoint{&u.ENDPT8}
	case 9:
		return usbEndpoint{&u.ENDPT9}
	case 10:
		return usbEndpoint{&u.ENDPT10}
	case 11:
		return usbEndpoint{&u.ENDPT11}
	case 12:
		return usbEndpoint{&u.ENDPT12}
	case 13:
		return usbEndpoint{&u.ENDPT13}
	case 14:
		return usbEndpoint{&u.ENDPT14}
	case 15:
		return usbEndpoint{&u.ENDPT15}
	default:
		panic("unknown endpoint")
	}
}

type usbEndpoint struct {
	*volatile.Register8
}

func (ep usbEndpoint) Stall() {
	ep.Set(nxp.USB0_ENDPT_EPSTALL | nxp.USB0_ENDPT_EPRXEN | nxp.USB0_ENDPT_EPTXEN | nxp.USB0_ENDPT_EPHSHK)
}

func (u *USB) setup() {
	USBMessages = append(USBMessages, fmt.Sprintf("setup request=%x type=%x value=%x index=%x", setup.bRequest, setup.bmRequestType, setup.wValue(), setup.wIndex))

	switch setup.bRequest {
	case usb_GET_STATUS:
		if setup.bmRequestType == 0x80 { // device
			ep0_transmit([]byte{0, 0})
			return

		} else if setup.bmRequestType == 0x82 { // endpoint
			i := setup.wIndex & 0x7F
			if i > usb_EndpointCount {
				// TODO: do we need to handle IN vs OUT here?
				break // stall
			}

			if u.Endpoint(i).HasBits(nxp.USB0_ENDPT_EPSTALL) {
				ep0_transmit([]byte{1, 0})
			} else {
				ep0_transmit([]byte{0, 0})
			}
			return
		}

	case usb_CLEAR_FEATURE:
		if setup.bmRequestType == 0x02 { // endpoint
			i := setup.wIndex & 0x7F
			if i > usb_EndpointCount || setup.wValue() != 0 {
				// TODO: do we need to handle IN vs OUT here?
				break // stall
			}

			u.Endpoint(i).ClearBits(nxp.USB0_ENDPT_EPSTALL)
			// TODO: do we need to clear the data toggle here?

			// send empty response
			ep0_transmit(nil)
			return
		}

	case usb_SET_FEATURE:
		if setup.bmRequestType == 0x02 { // endpoint
			i := setup.wIndex & 0x7F
			if i > usb_EndpointCount || setup.wValue() != 0 {
				// TODO: do we need to handle IN vs OUT here?
				break // stall
			}

			u.Endpoint(i).SetBits(nxp.USB0_ENDPT_EPSTALL)
			// TODO: do we need to clear the data toggle here?

			// send empty response
			ep0_transmit(nil)
			return
		}

	case usb_SET_ADDRESS:
		if setup.bmRequestType == 0x00 {
			// send empty response
			ep0_transmit(nil)
			return
		}

	case usb_GET_DESCRIPTOR:
		if setup.bmRequestType == 0x80 || setup.bmRequestType == 0x81 {
			USBMessages = append(USBMessages, "setup get descriptor")
			desc, ok := getUSBDescriptor(setup.wValue(), setup.wIndex)
			if !ok {
				break // stall
			}

			ep0_transmit(desc)
			return
		}

	case usb_GET_CONFIGURATION:
		if setup.bmRequestType == 0x80 {
			ep0_transmit([]byte{u.usb_configuration.Get()})
			return
		}

	case usb_SET_CONFIGURATION:
		if setup.bmRequestType == 0x00 {
			u.usb_configuration.Set(setup.wValueL)

			// clear all BDT entries, free any allocated memory...
			for i := 4; i < (usb_EndpointCount+1)*4; i++ {
				e := usbBufferDescriptorTable.Entry(i)
				if e.Owner() == usbBufferOwnedByUSBFS {
					// teensy core does not reset the descriptor
					e.Reset()
				}
			}

			// free all queued packets
			// N/A because GC?

			for i := uint16(1); i <= usb_EndpointCount; i++ {
				cfg := getUSBEndpointConfiguration(i - 1)
				u.Endpoint(i).Set(cfg)

				if cfg&nxp.USB0_ENDPT_EPRXEN != 0 {
					usbBufferDescriptorTable.Get(i, false, false).Set(make([]byte, usb_BUFFER_SIZE), false)
					usbBufferDescriptorTable.Get(i, false, true).Set(make([]byte, usb_BUFFER_SIZE), true)
				}

				usbBufferDescriptorTable.Get(i, true, false).Reset()
				usbBufferDescriptorTable.Get(i, true, true).Reset()
			}

			// send empty response
			ep0_transmit(nil)
			return
		}

	case usb_CDC_SET_LINE_CODING:
		if setup.bmRequestType == 0x21 {
			// send empty response
			ep0_transmit(nil)
			return
		}

	case usb_CDC_SET_CONTROL_LINE_STATE:
		if setup.bmRequestType == 0x21 {
			usb_cdc_line_rtsdtr_millis.Set(uint32(millisSinceBoot()))
			usb_cdc_line_rtsdtr.Set(uint32(setup.wValue()))

			// send empty response
			ep0_transmit(nil)
			return
		}

	case usb_CDC_SEND_BREAK:
		if setup.bmRequestType == 0x21 {
			// send empty response
			ep0_transmit(nil)
			return
		}
	}

	USBMessages = append(USBMessages, "setup stalled")
	u.Endpoint(0).Stall()
}

var (
	ep0_tx_remainder   []byte
	ep0_tx_bdt_bank    bool
	ep0_tx_data_toggle bool
)

func ep0_transmit_chunk(data []byte) (remainder []byte) {
	if len(data) > usb_EP0_SIZE {
		data, remainder = data[:usb_EP0_SIZE], data[usb_EP0_SIZE:]
	}

	USBMessages = append(USBMessages, fmt.Sprintf("ep0 transmit %d byte(s): %x", len(data), data))
	bd := usbBufferDescriptorTable.Get(0, true, ep0_tx_bdt_bank)
	bd.Set(data, ep0_tx_data_toggle)
	ep0_tx_data_toggle = !ep0_tx_data_toggle
	ep0_tx_bdt_bank = !ep0_tx_bdt_bank
	return
}

func ep0_transmit(data []byte) {
	// write to first bank
	data = ep0_transmit_chunk(data)
	if len(data) == 0 {
		return
	}

	// write to second bank
	data = ep0_transmit_chunk(data)
	if len(data) == 0 {
		return
	}

	// save remainder
	ep0_tx_remainder = data
}

//go:linkname ticks runtime.ticks
func ticks(int64)

var usb_cdc_line_rtsdtr_millis, usb_cdc_line_rtsdtr volatile.Register32
