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

// +build nxp,mk66f18

package runtime

import (
	"device/arm"
	"device/nxp"
	"machine"
	"runtime/volatile"
)

func init() {
	initCLK()
	// machine.UART0.Configure(machine.UARTConfig{})
}

func initCLK() {
}

func putchar(c byte) {
	// machine.UART0.WriteByte(c)
}

// ???
const asyncScheduler = false

// microseconds per tick
const tickMicros = 1000

// number of ticks since boot
var tickMilliCount uint32

//go:export SysTick_Handler
func tickHandler() {
	volatile.StoreUint32(&tickMilliCount, volatile.LoadUint32(&tickMilliCount)+1)
}

// ticks are in microseconds
func ticks() timeUnit {
	arm.Asm("CPSID i")
	current := nxp.SysTick.CVR.Get()
	count := tickMilliCount
	istatus := nxp.SystemControl.ICSR.Get()
	arm.Asm("CPSIE i")

	if istatus&nxp.SystemControl_ICSR_PENDSTSET != 0 && current > 50 {
		count++
	}

	current = ((machine.CPUFrequency() / tickMicros) - 1) - current
	return timeUnit(count*tickMicros + current/(machine.CPUFrequency()/1000000))
}

// sleepTicks spins for a number of microseconds
func sleepTicks(d timeUnit) {
	// TODO actually sleep

	if d <= 0 {
		return
	}

	start := ticks()
	ms := d / 1000

	for {
		for ticks()-start >= 1000 {
			ms--
			if ms <= 0 {
				return
			}
			start += 1000
		}
		Gosched()
	}
}

func Sleep(d int64) {
	sleepTicks(timeUnit(d))
}
