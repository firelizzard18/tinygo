// +build nxp

package runtime

type timeUnit int64

//go:export Reset_Handler
func main() {
	// preinit()
	// initAll()
	callMain()
	abort()
}
