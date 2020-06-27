// +build nxp,mk66f18,teensy36

package machine

import (
	"device/arm"
	"device/nxp"
	"runtime/volatile"
	"strconv"
	"sync/atomic"
	"unsafe"
)

// F_CPU = 180 MHz
// F_PLL = 180 MHz
// F_BUS =  60 MHz
// clock =  16 MHz
// F_MEM = 25714286 Hz

// CPUFrequency returns the frequency of the ARM core clock (180MHz)
func CPUFrequency() uint32 { return 180000000 }

// ClockFrequency returns the frequency of the external oscillator (16MHz)
func ClockFrequency() uint32 { return 16000000 }

// LED on the Teensy
const LED = PC05

const (
	usb_STRING_MANUFACTURER = "Teensy"
	usb_STRING_PRODUCT      = "Teensy 3.6 USB Serial"

	usb_DEVICE_CLASS    = 0x02
	usb_DEVICE_SUBCLASS = 0
	usb_DEVICE_PROTOCOL = 0

	usb_EP0_SIZE    = 64
	usb_BUFFER_SIZE = usb_EP0_SIZE

	usb_VID        = 0x16C0
	usb_PID        = 0x0483 // teensy usb serial
	usb_BCD_DEVICE = 0x0277 // teensy 3.6

	// rev should appear as 0x0277

	usb_InterfaceID_CDC_Status = 0
	usb_InterfaceID_CDC_Data   = 1
	usb_InterfaceCount         = 2

	usb_EndpointID_CDC_ACM       = 2
	usb_EndpointID_CDC_RX        = 3
	usb_EndpointID_CDC_TX        = 4
	usb_EndpointCount_CDC_Status = 1
	usb_EndpointCount_CDC_Data   = 2
	usb_EndpointCount            = 4
)

var usb_STRING_SERIAL string

// Port E is unavailable pending resolution of
// https://github.com/tinygo-org/tinygo/issues/1190

// digital IO
const (
	D00 = PB16
	D01 = PB17
	D02 = PD00
	D03 = PA12
	D04 = PA13
	D05 = PD07
	D06 = PD04
	D07 = PD02
	D08 = PD03
	D09 = PC03
	D10 = PC04
	D11 = PC06
	D12 = PC07
	D13 = PC05
	D14 = PD01
	D15 = PC00
	D16 = PB00
	D17 = PB01
	D18 = PB03
	D19 = PB02
	D20 = PD05
	D21 = PD06
	D22 = PC01
	D23 = PC02
	D24 = PE26
	D25 = PA05
	D26 = PA14
	D27 = PA15
	D28 = PA16
	D29 = PB18
	D30 = PB19
	D31 = PB10
	D32 = PB11
	D33 = PE24
	D34 = PE25
	D35 = PC08
	D36 = PC09
	D37 = PC10
	D38 = PC11
	D39 = PA17
	D40 = PA28
	D41 = PA29
	D42 = PA26
	D43 = PB20
	D44 = PB22
	D45 = PB23
	D46 = PB21
	D47 = PD08
	D48 = PD09
	D49 = PB04
	D50 = PB05
	D51 = PD14
	D52 = PD13
	D53 = PD12
	D54 = PD15
	D55 = PD11
	D56 = PE10
	D57 = PE11
	D58 = PE00
	D59 = PE01
	D60 = PE02
	D61 = PE03
	D62 = PE04
	D63 = PE05
)

var (
	TeensyUART1 = &UART0
	TeensyUART2 = &UART1
	TeensyUART3 = &UART2
	TeensyUART4 = &UART3
	TeensyUART5 = &UART4
)

const (
	defaultUART0RX = D00
	defaultUART0TX = D01
	defaultUART1RX = D09
	defaultUART1TX = D10
	defaultUART2RX = D07
	defaultUART2TX = D08
	defaultUART3RX = D31
	defaultUART3TX = D32
	defaultUART4RX = D34
	defaultUART4TX = D33
)

func EnterBootloader() {
	arm.Asm("bkpt")
}

//go:linkname sleepTicks runtime.sleepTicks
func sleepTicks(int64)

var initted int32

func init() {
	atomic.StoreInt32(&initted, 0)
}

func InitPlatform() {
	if !atomic.CompareAndSwapInt32(&initted, 0, 1) {
		return
	}

	// for background about this startup delay, please see these conversations
	// https://forum.pjrc.com/threads/36606-startup-time-(400ms)?p=113980&viewfull=1#post113980
	// https://forum.pjrc.com/threads/31290-Teensey-3-2-Teensey-Loader-1-24-Issues?p=87273&viewfull=1#post87273
	const millis = 1000
	sleepTicks(25 * millis) // 50 for TeensyDuino < 142
	usb_STRING_SERIAL = readSerialNumber()
	setupUSB()
	sleepTicks(275 * millis) // 350 for TeensyDuino < 142
}

func getUSBDescriptor(value, index uint16) ([]byte, bool) {
	// from: usb_desc.c, usb_desc.h

	if value == 0x0100 && index == 0x0000 {
		return NewDeviceDescriptor(
			usb_DEVICE_CLASS, usb_DEVICE_SUBCLASS, usb_DEVICE_PROTOCOL,
			usb_EP0_SIZE,
			usb_VID, usb_PID, usb_BCD_DEVICE,
			usb_IMANUFACTURER, usb_IPRODUCT, usb_ISERIAL,
			1,
		).Bytes(), true
	}

	if value == 0x0200 && index == 0x0000 {
		const cdcACMSize = 16
		const cdcRXSize = 64
		const cdcTXSize = 64

		b := []byte{
			configDescriptorSize,              // bLength
			usb_CONFIGURATION_DESCRIPTOR_TYPE, // bDescriptorType
			0,                                 // wTotalLength LSB
			0,                                 //              MSB
			usb_InterfaceCount,                // bNumInterfaces
			1,                                 // bConfigurationValue
			0,                                 // iConfiguration
			0xC0,                              // bmAttributes
			50,                                // bMaxPower

			// Interface Descriptor, USB spec 9.6.5, page 267-269, Table 9-12
			interfaceDescriptorSize,               // bLength
			usb_INTERFACE_DESCRIPTOR_TYPE,         // bDescriptorType
			usb_InterfaceID_CDC_Status,            // bInterfaceNumber
			0,                                     // bAlternateSetting
			usb_EndpointCount_CDC_Status,          // bNumEndpoints
			usb_CDC_COMMUNICATION_INTERFACE_CLASS, // bInterfaceClass
			usb_CDC_ABSTRACT_CONTROL_MODEL,        // bInterfaceSubClass
			0x01,                                  // bInterfaceProtocol
			0,                                     // iInterface

			// CDC Header Functional Descriptor, CDC Spec 5.2.3.1, Table 26
			5,          // bFunctionLength
			0x24,       // bDescriptorType
			0x00,       // bDescriptorSubtype
			0x10, 0x01, // bcdCDC

			// Call Management Functional Descriptor, CDC Spec 5.2.3.2, Table 27
			5,    // bFunctionLength
			0x24, // bDescriptorType
			0x01, // bDescriptorSubtype
			0x01, // bmCapabilities
			1,    // bDataInterface

			// Abstract Control Management Functional Descriptor, CDC Spec 5.2.3.3, Table 28
			4,    // bFunctionLength
			0x24, // bDescriptorType
			0x02, // bDescriptorSubtype
			0x06, // bmCapabilities

			// Union Functional Descriptor, CDC Spec 5.2.3.8, Table 33
			5,                          // bFunctionLength
			0x24,                       // bDescriptorType
			0x06,                       // bDescriptorSubtype
			usb_InterfaceID_CDC_Status, // bMasterInterface
			usb_InterfaceID_CDC_Data,   // bSlaveInterface0

			// Endpoint Descriptor, USB spec 9.6.6, page 269-271, Table 9-13
			endpointDescriptorSize,        // bLength
			usb_ENDPOINT_DESCRIPTOR_TYPE,  // bDescriptorType
			usb_EndpointID_CDC_ACM | 0x80, // bEndpointAddress
			0x03,                          // bmAttributes (0x03=intr)
			cdcACMSize,                    // wMaxPacketSize LSB
			0,                             //                MSB
			64,                            // bInterval

			// Interface Descriptor, USB spec 9.6.5, page 267-269, Table 9-12
			interfaceDescriptorSize,       // bLength
			usb_INTERFACE_DESCRIPTOR_TYPE, // bDescriptorType
			usb_InterfaceID_CDC_Data,      // bInterfaceNumber
			0,                             // bAlternateSetting
			usb_EndpointCount_CDC_Data,    // bNumEndpoints
			usb_CDC_DATA_INTERFACE_CLASS,  // bInterfaceClass
			0x00,                          // bInterfaceSubClass
			0x00,                          // bInterfaceProtocol
			0,                             // iInterface

			// Endpoint Descriptor, USB spec 9.6.6, page 269-271, Table 9-13
			endpointDescriptorSize,       // bLength
			usb_ENDPOINT_DESCRIPTOR_TYPE, // bDescriptorType
			usb_EndpointID_CDC_RX,        // bEndpointAddress
			0x02,                         // bmAttributes (0x02=bulk)
			cdcRXSize,                    // wMaxPacketSize LSB
			0,                            //                MSB
			0,                            // bInterval

			// Endpoint Descriptor, USB spec 9.6.6, page 269-271, Table 9-13
			endpointDescriptorSize,       // bLength
			usb_ENDPOINT_DESCRIPTOR_TYPE, // bDescriptorType
			usb_EndpointID_CDC_TX | 0x80, // bEndpointAddress
			0x02,                         // bmAttributes (0x02=bulk)
			cdcTXSize,                    // wMaxPacketSize LSB
			0,                            //                MSB
			0,                            // bInterval
		}

		b[2] = byte(len(b))      // wTotalLength LSB
		b[3] = byte(len(b) >> 8) //              MSB
		return b, true
	}

	if value == 0x0300 && index == 0x0000 {
		return []byte{4, 3, 0x04, 0x09}, true
	}

	if value == 0x0301 && index == 0x0409 {
		return newStringDescriptor(usb_STRING_MANUFACTURER), true
	}

	if value == 0x0302 && index == 0x0409 {
		return newStringDescriptor(usb_STRING_PRODUCT), true
	}

	if value == 0x0303 && index == 0x0409 {
		return newStringDescriptor(usb_STRING_SERIAL), true
	}

	return nil, false
}

func getUSBEndpointConfiguration(i uint16) uint8 {
	switch i {
	case 1:
		return nxp.USB0_ENDPT_EPCTLDIS | nxp.USB0_ENDPT_EPTXEN | nxp.USB0_ENDPT_EPHSHK
	case 2:
		return nxp.USB0_ENDPT_EPCTLDIS | nxp.USB0_ENDPT_EPRXEN | nxp.USB0_ENDPT_EPHSHK
	case 3:
		return nxp.USB0_ENDPT_EPCTLDIS | nxp.USB0_ENDPT_EPTXEN | nxp.USB0_ENDPT_EPHSHK
	default:
		return 0
	}
}

func readSerialNumber() string {
	// from: usb_init_serialnumber

	flags := arm.DisableInterrupts()
	nxp.KinetisHSRunDisable()
	nxp.FTFE.FSTAT.Set(nxp.FTFE_FSTAT_RDCOLERR | nxp.FTFE_FSTAT_ACCERR | nxp.FTFE_FSTAT_FPVIOL)
	((*volatile.Register32)(unsafe.Pointer(&nxp.FTFE.FCCOB3))).Set(0x41070000)
	nxp.FTFE.FSTAT.Set(nxp.FTFE_FSTAT_CCIF)
	for !nxp.FTFE.FSTAT.HasBits(nxp.FTFE_FSTAT_CCIF) {
	} // wait
	num := ((*volatile.Register32)(unsafe.Pointer(&nxp.FTFE.FCCOBB))).Get()
	nxp.KinetisHSRunEnable()
	arm.EnableInterrupts(flags)

	// add extra zero to work around OS-X CDC-ACM driver bug
	if num < 10000000 {
		num *= 10
	}

	// can't allocate during startup
	return strconv.FormatInt(int64(num), 10)
}
