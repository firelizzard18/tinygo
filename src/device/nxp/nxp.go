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

// +build nxp

package nxp

const (
	_SMC_PMCTRL_RUN   = 0 << SMC_PMCTRL_RUNM_Pos
	_SMC_PMCTRL_HSRUN = 3 << SMC_PMCTRL_RUNM_Pos
	_SMC_PMSTAT_RUN   = 0x01 << SMC_PMSTAT_PMSTAT_Pos
	_SMC_PMSTAT_HSRUN = 0x80 << SMC_PMSTAT_PMSTAT_Pos
)

func KinetisHSRunDisable() bool {
	// from: kinetis_hsrun_disable

	if SMC.PMSTAT.Get() != _SMC_PMSTAT_HSRUN {
		return false
	}

	// First, reduce the CPU clock speed, but do not change
	// the peripheral speed (F_BUS).  Serial1 & Serial2 baud
	// rates will be impacted, but most other peripherals
	// will continue functioning at the same speed.
	SIM.CLKDIV1.Set((2 << SIM_CLKDIV1_OUTDIV1_Pos) | (2 << SIM_CLKDIV1_OUTDIV2_Pos) | (2 << SIM_CLKDIV1_OUTDIV1_Pos) | (8 << SIM_CLKDIV1_OUTDIV4_Pos))

	// Then turn off HSRUN mode
	SMC.PMCTRL.Set(_SMC_PMCTRL_RUN)

	// Wait for HSRUN to end
	for SMC.PMSTAT.Get() == _SMC_PMSTAT_HSRUN {
	}

	return false
}

func KinetisHSRunEnable() bool {
	// from: kinetis_hsrun_enable

	if SMC.PMSTAT.Get() != _SMC_PMSTAT_RUN {
		return false
	}

	// Turn HSRUN mode on
	SMC.PMCTRL.Set(_SMC_PMCTRL_HSRUN)

	// Wait for HSRUN to start
	for SMC.PMSTAT.Get() != _SMC_PMSTAT_HSRUN {
	}

	// Then configure clock for full speed
	SIM.CLKDIV1.Set((0 << SIM_CLKDIV1_OUTDIV1_Pos) | (2 << SIM_CLKDIV1_OUTDIV2_Pos) | (0 << SIM_CLKDIV1_OUTDIV1_Pos) | (6 << SIM_CLKDIV1_OUTDIV4_Pos))

	return true
}
