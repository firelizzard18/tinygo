// +build sam nrf52840 nxp,mk66f18

package machine

import (
	"errors"
)

var (
	errUSBCDCBufferEmpty      = errors.New("USB-CDC buffer empty")
	errUSBCDCWriteByteTimeout = errors.New("USB-CDC write byte timeout")
)

// USBCDC is the serial interface that works over the USB port.
// To implement the USBCDC interface for a board, you must declare a concrete type as follows:
//
// 		type USBCDC struct {
// 			Buffer *RingBuffer
// 		}
//
// You can also add additional members to this struct depending on your implementation,
// but the *RingBuffer is required.
// When you are declaring the USBCDC for your board, make sure that you also declare the
// RingBuffer using the NewRingBuffer() function:
//
//		USBCDC{Buffer: NewRingBuffer()}
//

// Read from the RX buffer.
func (usbcdc USBCDC) Read(data []byte) (n int, err error) {
	// check if RX buffer is empty
	size := usbcdc.Buffered()
	if size == 0 {
		return 0, nil
	}

	// Make sure we do not read more from buffer than the data slice can hold.
	if len(data) < size {
		size = len(data)
	}

	// only read number of bytes used from buffer
	for i := 0; i < size; i++ {
		v, _ := usbcdc.ReadByte()
		data[i] = v
	}

	return size, nil
}

// Write data to the USBCDC.
func (usbcdc USBCDC) Write(data []byte) (n int, err error) {
	for _, v := range data {
		usbcdc.WriteByte(v)
	}
	return len(data), nil
}

// ReadByte reads a single byte from the RX buffer.
// If there is no data in the buffer, returns an error.
func (usbcdc USBCDC) ReadByte() (byte, error) {
	// check if RX buffer is empty
	buf, ok := usbcdc.Buffer.Get()
	if !ok {
		return 0, errUSBCDCBufferEmpty
	}
	return buf, nil
}

// Buffered returns the number of bytes currently stored in the RX buffer.
func (usbcdc USBCDC) Buffered() int {
	return int(usbcdc.Buffer.Used())
}

// Receive handles adding data to the UART's data buffer.
// Usually called by the IRQ handler for a machine.
func (usbcdc USBCDC) Receive(data byte) {
	usbcdc.Buffer.Put(data)
}
