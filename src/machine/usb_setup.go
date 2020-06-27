// +build sam nrf52840

package machine

import (
	"errors"
	"runtime/volatile"
)

// MSCDescriptor is not used yet.
type MSCDescriptor struct {
	msc InterfaceDescriptor
	in  EndpointDescriptor
	out EndpointDescriptor
}

type cdcLineInfo struct {
	dwDTERate   uint32
	bCharFormat uint8
	bParityType uint8
	bDataBits   uint8
	lineState   uint8
}

// usbDeviceDescBank is the USB device endpoint descriptor.
// typedef struct {
// 	__IO USB_DEVICE_ADDR_Type      ADDR;        /**< \brief Offset: 0x000 (R/W 32) DEVICE_DESC_BANK Endpoint Bank, Adress of Data Buffer */
// 	__IO USB_DEVICE_PCKSIZE_Type   PCKSIZE;     /**< \brief Offset: 0x004 (R/W 32) DEVICE_DESC_BANK Endpoint Bank, Packet Size */
// 	__IO USB_DEVICE_EXTREG_Type    EXTREG;      /**< \brief Offset: 0x008 (R/W 16) DEVICE_DESC_BANK Endpoint Bank, Extended */
// 	__IO USB_DEVICE_STATUS_BK_Type STATUS_BK;   /**< \brief Offset: 0x00A (R/W  8) DEVICE_DESC_BANK Enpoint Bank, Status of Bank */
// 		 RoReg8                    Reserved1[0x5];
//   } UsbDeviceDescBank;
type usbDeviceDescBank struct {
	ADDR      volatile.Register32
	PCKSIZE   volatile.Register32
	EXTREG    volatile.Register16
	STATUS_BK volatile.Register8
	_reserved [5]volatile.Register8
}

type usbDeviceDescriptor struct {
	DeviceDescBank [2]usbDeviceDescBank
}

// sendDescriptor creates and sends the various USB descriptor types that
// can be requested by the host.
func sendDescriptor(setup usbSetup) {
	switch setup.wValueH {
	case usb_CONFIGURATION_DESCRIPTOR_TYPE:
		sendConfiguration(setup)
		return
	case usb_DEVICE_DESCRIPTOR_TYPE:
		// composite descriptor
		dd := NewDeviceDescriptor(0xef, 0x02, 0x01, 64, usb_VID, usb_PID, usb_BCD_DEVICE, usb_IMANUFACTURER, usb_IPRODUCT, usb_ISERIAL, 1)
		l := deviceDescriptorSize
		if setup.wLength < deviceDescriptorSize {
			l = int(setup.wLength)
		}
		sendUSBPacket(0, dd.Bytes()[:l])
		return

	case usb_STRING_DESCRIPTOR_TYPE:
		switch setup.wValueL {
		case 0:
			b := []byte{0x04, 0x03, 0x09, 0x04}
			sendUSBPacket(0, b)

		case usb_IPRODUCT:
			b := make([]byte, (len(usb_STRING_PRODUCT)<<1)+2)
			strToUTF16LEDescriptor(usb_STRING_PRODUCT, b)
			sendUSBPacket(0, b)

		case usb_IMANUFACTURER:
			b := make([]byte, (len(usb_STRING_MANUFACTURER)<<1)+2)
			strToUTF16LEDescriptor(usb_STRING_MANUFACTURER, b)
			sendUSBPacket(0, b)

		case usb_ISERIAL:
			// TODO: allow returning a product serial number
			sendZlp()
		}
		return
	}

	// do not know how to handle this message, so return zero
	sendZlp()
	return
}

// sendConfiguration creates and sends the configuration packet to the host.
func sendConfiguration(setup usbSetup) {
	if setup.wLength == 9 {
		sz := uint16(configDescriptorSize + cdcSize)
		config := NewConfigDescriptor(sz, 2)
		sendUSBPacket(0, config.Bytes())
	} else {
		iad := NewIADDescriptor(0, 2, usb_CDC_COMMUNICATION_INTERFACE_CLASS, usb_CDC_ABSTRACT_CONTROL_MODEL, 0)

		cif := NewInterfaceDescriptor(usb_CDC_ACM_INTERFACE, 1, usb_CDC_COMMUNICATION_INTERFACE_CLASS, usb_CDC_ABSTRACT_CONTROL_MODEL, 0)

		header := NewCDCCSInterfaceDescriptor(usb_CDC_HEADER, usb_CDC_V1_10&0xFF, (usb_CDC_V1_10>>8)&0x0FF)

		controlManagement := NewACMFunctionalDescriptor(usb_CDC_ABSTRACT_CONTROL_MANAGEMENT, 6)

		functionalDescriptor := NewCDCCSInterfaceDescriptor(usb_CDC_UNION, usb_CDC_ACM_INTERFACE, usb_CDC_DATA_INTERFACE)

		callManagement := NewCMFunctionalDescriptor(usb_CDC_CALL_MANAGEMENT, 1, 1)

		cifin := NewEndpointDescriptor((usb_CDC_ENDPOINT_ACM | usbEndpointIn), usb_ENDPOINT_TYPE_INTERRUPT, 0x10, 0x10)

		dif := NewInterfaceDescriptor(usb_CDC_DATA_INTERFACE, 2, usb_CDC_DATA_INTERFACE_CLASS, 0, 0)

		out := NewEndpointDescriptor((usb_CDC_ENDPOINT_OUT | usbEndpointOut), usb_ENDPOINT_TYPE_BULK, usbEndpointPacketSize, 0)

		in := NewEndpointDescriptor((usb_CDC_ENDPOINT_IN | usbEndpointIn), usb_ENDPOINT_TYPE_BULK, usbEndpointPacketSize, 0)

		cdc := NewCDCDescriptor(iad,
			cif,
			header,
			controlManagement,
			functionalDescriptor,
			callManagement,
			cifin,
			dif,
			out,
			in)

		sz := uint16(configDescriptorSize + cdcSize)
		config := NewConfigDescriptor(sz, 2)

		buf := make([]byte, 0, sz)
		buf = append(buf, config.Bytes()...)
		buf = append(buf, cdc.Bytes()...)

		sendUSBPacket(0, buf)
	}
}
