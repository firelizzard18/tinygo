// +build nxp,mk66f18

package runtime

func init() {
	initCLK()
	// machine.UART0.Configure(machine.UARTConfig{})
}

func initCLK() {
}

func putchar(c byte) {
	// machine.UART0.WriteByte(c)
}
