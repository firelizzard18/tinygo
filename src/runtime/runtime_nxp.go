// +build nxp

package runtime

type timeUnit int64

const tickMicros = 1024 * 32

func ticks() timeUnit {
	// TODO
	return 0
}

const asyncScheduler = false

func sleepTicks(d timeUnit) {
	// TODO
}

//go:export Reset_Handler
func main() {
	// preinit()
	// initAll()
	callMain()
	abort()
}
