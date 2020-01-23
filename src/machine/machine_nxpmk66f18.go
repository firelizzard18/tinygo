// +build nxp,mk66f18

package machine

// Set changes the value of the GPIO pin. The pin must be configured as output.
func (p Pin) Set(value bool) {
	// if value { // set bits
	// 	port, mask := p.PortMaskSet()
	// 	port.Set(mask)
	// } else { // clear bits
	// 	port, mask := p.PortMaskClear()
	// 	port.Set(mask)
	// }
}

// Get returns the current value of a GPIO pin.
func (p Pin) Get() bool {
	// if p < 8 {
	// 	val := avr.PIND.Get() & (1 << uint8(p))
	// 	return (val > 0)
	// } else if p < 14 {
	// 	val := avr.PINB.Get() & (1 << uint8(p-8))
	// 	return (val > 0)
	// } else {
	// 	val := avr.PINC.Get() & (1 << uint8(p-14))
	// 	return (val > 0)
	// }
	return false
}
