// +build nxp

package machine

import (
	"device/arm"
	"device/nxp"
)

type PinMode uint8

// Stop enters STOP (deep sleep) mode
func Stop() {
	// set SLEEPDEEP to enable deep sleep
	arm.SCB.SCR.SetBits(nxp.SystemControl_SCR_SLEEPDEEP)

	// enter STOP mode
	arm.Asm("wfi")
}

// Wait enters WAIT (sleep) mode
func Wait() {
	// clear SLEEPDEEP bit to disable deep sleep
	arm.SCB.SCR.ClearBits(nxp.SystemControl_SCR_SLEEPDEEP)

	// enter WAIT mode
	arm.Asm("wfi")
}
