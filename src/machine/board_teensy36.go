// +build nxp,mk66f18,teensy36

package machine

func CPUFrequency() uint32 {
	return 180000000
}

// LED on the Teensy
const LED Pin = 13
