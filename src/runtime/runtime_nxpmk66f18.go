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
	"runtime/interrupt"
	"runtime/volatile"
)

const (
	watchdogUnlockSequence1 = 0xC520
	watchdogUnlockSequence2 = 0xD928

	_DEFAULT_FTM_MOD      = 61440 - 1
	_DEFAULT_FTM_PRESCALE = 1
)

var (
	_SIM_SOPT2_IRC48SEL = nxp.SIM_SOPT2_PLLFLLSEL(3)
	_SMC_PMCTRL_HSRUN   = nxp.SMC_PMCTRL_RUNM(3)
	_SMC_PMSTAT_HSRUN   = nxp.SMC_PMSTAT_PMSTAT(0x80)
)

//go:export Reset_Handler
func main() {
	initSystem()
	arm.Asm("CPSIE i")
	initInternal()

	initAll()
	callMain()
	abort()
}

func initSystem() {
	// from: ResetHandler

	nxp.WDOG.UNLOCK.Set(watchdogUnlockSequence1)
	nxp.WDOG.UNLOCK.Set(watchdogUnlockSequence2)
	arm.Asm("nop")
	arm.Asm("nop")
	// TODO: hook for overriding? 'startupEarlyHook'
	nxp.WDOG.STCTRLH.Set(nxp.WDOG_STCTRLH_ALLOWUPDATE)

	// enable clocks to always-used peripherals
	nxp.SIM.SCGC3.Set(nxp.SIM_SCGC3_ADC1 | nxp.SIM_SCGC3_FTM2 | nxp.SIM_SCGC3_FTM3)
	nxp.SIM.SCGC5.Set(0x00043F82) // clocks active to all GPIO
	nxp.SIM.SCGC6.Set(nxp.SIM_SCGC6_RTC | nxp.SIM_SCGC6_FTM0 | nxp.SIM_SCGC6_FTM1 | nxp.SIM_SCGC6_ADC0 | nxp.SIM_SCGC6_FTF)
	nxp.SystemControl.CPACR.Set(0x00F00000)
	nxp.LMEM.PCCCR.Set(0x85000003)

	// release I/O pins hold, if we woke up from VLLS mode
	if nxp.PMC.REGSC.HasBits(nxp.PMC_REGSC_ACKISO) {
		nxp.PMC.REGSC.SetBits(nxp.PMC_REGSC_ACKISO)
	}

	// since this is a write once register, make it visible to all F_CPU's
	// so we can into other sleep modes in the future at any speed
	nxp.SMC.PMPROT.Set(nxp.SMC_PMPROT_AHSRUN | nxp.SMC_PMPROT_AVLP | nxp.SMC_PMPROT_ALLS | nxp.SMC_PMPROT_AVLLS)

	preinit()

	// copy the vector table to RAM default all interrupts to medium priority level
	// for (i=0; i < NVIC_NUM_INTERRUPTS + 16; i++) _VectorsRam[i] = _VectorsFlash[i];
	for i := uint32(0); i <= nxp.IRQ_max; i++ {
		arm.SetPriority(i, 128)
	}
	// SCB_VTOR = (uint32_t)_VectorsRam;	// use vector table in RAM

	// hardware always starts in FEI mode
	//  C1[CLKS] bits are written to 00
	//  C1[IREFS] bit is written to 1
	//  C6[PLLS] bit is written to 0
	// MCG_SC[FCDIV] defaults to divide by two for internal ref clock
	// I tried changing MSG_SC to divide by 1, it didn't work for me
	// enable capacitors for crystal
	nxp.OSC.CR.Set(nxp.OSC_CR_SC8P | nxp.OSC_CR_SC2P | nxp.OSC_CR_ERCLKEN)
	// enable osc, 8-32 MHz range, low power mode
	nxp.MCG.C2.Set(uint8(nxp.MCG_C2_RANGE(2) | nxp.MCG_C2_EREFS))
	// switch to crystal as clock source, FLL input = 16 MHz / 512
	nxp.MCG.C1.Set(uint8(nxp.MCG_C1_CLKS(2) | nxp.MCG_C1_FRDIV(4)))
	// wait for crystal oscillator to begin
	for !nxp.MCG.S.HasBits(nxp.MCG_S_OSCINIT0) {
	}
	// wait for FLL to use oscillator
	for nxp.MCG.S.HasBits(nxp.MCG_S_IREFST) {
	}
	// wait for MCGOUT to use oscillator
	for (nxp.MCG.S.Get() & nxp.MCG_S_CLKST_Msk) != nxp.MCG_S_CLKST(2) {
	}

	// now in FBE mode
	//  C1[CLKS] bits are written to 10
	//  C1[IREFS] bit is written to 0
	//  C1[FRDIV] must be written to divide xtal to 31.25-39 kHz
	//  C6[PLLS] bit is written to 0
	//  C2[LP] is written to 0
	// we need faster than the crystal, turn on the PLL (F_CPU > 120000000)
	nxp.SMC.PMCTRL.Set(_SMC_PMCTRL_HSRUN) // enter HSRUN mode
	for nxp.SMC.PMSTAT.Get() != _SMC_PMSTAT_HSRUN {
	} // wait for HSRUN
	nxp.MCG.C5.Set(nxp.MCG_C5_PRDIV(1))
	nxp.MCG.C6.Set(nxp.MCG_C6_PLLS | nxp.MCG_C6_VDIV(29))

	// wait for PLL to start using xtal as its input
	for !nxp.MCG.S.HasBits(nxp.MCG_S_PLLST) {
	}
	// wait for PLL to lock
	for !nxp.MCG.S.HasBits(nxp.MCG_S_LOCK0) {
	}
	// now we're in PBE mode

	// now program the clock dividers
	// config divisors: 180 MHz core, 60 MHz bus, 25.7 MHz flash, USB = IRC48M
	nxp.SIM.CLKDIV1.Set(nxp.SIM_CLKDIV1_OUTDIV1(0) | nxp.SIM_CLKDIV1_OUTDIV2(2) | nxp.SIM_CLKDIV1_OUTDIV4(6))
	nxp.SIM.CLKDIV2.Set(nxp.SIM_CLKDIV2_USBDIV(0))

	// switch to PLL as clock source, FLL input = 16 MHz / 512
	nxp.MCG.C1.Set(nxp.MCG_C1_CLKS(0) | nxp.MCG_C1_FRDIV(4))
	// wait for PLL clock to be used
	for (nxp.MCG.S.Get() & nxp.MCG_S_CLKST_Msk) != nxp.MCG_S_CLKST(3) {
	}
	// now we're in PEE mode
	// trace is CPU clock, CLKOUT=OSCERCLK0
	// USB uses IRC48
	nxp.SIM.SOPT2.Set(nxp.SIM_SOPT2_USBSRC | _SIM_SOPT2_IRC48SEL | nxp.SIM_SOPT2_TRACECLKSEL | nxp.SIM_SOPT2_CLKOUTSEL(6))

	// If the RTC oscillator isn't enabled, get it started.  For Teensy 3.6
	// we don't do this early.  See comment above about slow rising power.
	if !nxp.RTC.CR.HasBits(nxp.RTC_CR_OSCE) {
		nxp.RTC.SR.Set(0)
		nxp.RTC.CR.Set(nxp.RTC_CR_SC16P | nxp.RTC_CR_SC4P | nxp.RTC_CR_OSCE)
	}

	// initialize the SysTick counter
	nxp.SysTick.RVR.Set(cyclesPerMilli - 1)
	nxp.SysTick.CVR.Set(0)
	nxp.SysTick.CSR.Set(nxp.SysTick_CSR_CLKSOURCE | nxp.SysTick_CSR_TICKINT | nxp.SysTick_CSR_ENABLE)
	nxp.SystemControl.SHPR3.Set(nxp.SystemControl_SHPR3_PRI_15(32) | nxp.SystemControl_SHPR3_PRI_14(32)) // set systick and pendsv priority to 32
}

func initInternal() {
	// from: _init_Teensyduino_internal_
	// arm.EnableIRQ(nxp.IRQ_PORTA)
	// arm.EnableIRQ(nxp.IRQ_PORTB)
	// arm.EnableIRQ(nxp.IRQ_PORTC)
	// arm.EnableIRQ(nxp.IRQ_PORTD)
	// arm.EnableIRQ(nxp.IRQ_PORTE)

	nxp.FTM0.CNT.Set(0)
	nxp.FTM0.MOD.Set(_DEFAULT_FTM_MOD)
	nxp.FTM0.C0SC.Set(0x28) // MSnB:MSnA = 10, ELSnB:ELSnA = 10
	nxp.FTM0.C1SC.Set(0x28)
	nxp.FTM0.C2SC.Set(0x28)
	nxp.FTM0.C3SC.Set(0x28)
	nxp.FTM0.C4SC.Set(0x28)
	nxp.FTM0.C5SC.Set(0x28)
	nxp.FTM0.C6SC.Set(0x28)
	nxp.FTM0.C7SC.Set(0x28)

	nxp.FTM3.C0SC.Set(0x28)
	nxp.FTM3.C1SC.Set(0x28)
	nxp.FTM3.C2SC.Set(0x28)
	nxp.FTM3.C3SC.Set(0x28)
	nxp.FTM3.C4SC.Set(0x28)
	nxp.FTM3.C5SC.Set(0x28)
	nxp.FTM3.C6SC.Set(0x28)
	nxp.FTM3.C7SC.Set(0x28)

	nxp.FTM0.SC.Set(nxp.FTM_SC_CLKS(1) | nxp.FTM_SC_PS(_DEFAULT_FTM_PRESCALE))
	nxp.FTM1.CNT.Set(0)
	nxp.FTM1.MOD.Set(_DEFAULT_FTM_MOD)
	nxp.FTM1.C0SC.Set(0x28)
	nxp.FTM1.C1SC.Set(0x28)
	nxp.FTM1.SC.Set(nxp.FTM_SC_CLKS(1) | nxp.FTM_SC_PS(_DEFAULT_FTM_PRESCALE))

	// nxp.FTM2.CNT.Set(0)
	// nxp.FTM2.MOD.Set(_DEFAULT_FTM_MOD)
	// nxp.FTM2.C0SC.Set(0x28)
	// nxp.FTM2.C1SC.Set(0x28)
	// nxp.FTM2.SC.Set(nxp.FTM_SC_CLKS(1) | nxp.FTM_SC_PS(_DEFAULT_FTM_PRESCALE))

	nxp.FTM3.CNT.Set(0)
	nxp.FTM3.MOD.Set(_DEFAULT_FTM_MOD)
	nxp.FTM3.C0SC.Set(0x28)
	nxp.FTM3.C1SC.Set(0x28)
	nxp.FTM3.SC.Set(nxp.FTM_SC_CLKS(1) | nxp.FTM_SC_PS(_DEFAULT_FTM_PRESCALE))

	nxp.SIM.SCGC2.SetBits(nxp.SIM_SCGC2_TPM1)
	nxp.SIM.SOPT2.SetBits(nxp.SIM_SOPT2_TPMSRC(2))
	nxp.TPM1.CNT.Set(0)
	nxp.TPM1.MOD.Set(32767)
	nxp.TPM1.C0SC.Set(0x28)
	nxp.TPM1.C1SC.Set(0x28)
	nxp.TPM1.SC.Set(nxp.FTM_SC_CLKS(1) | nxp.FTM_SC_PS(0))

	// configure the low-power timer
	nxp.SIM.SCGC5.SetBits(nxp.SIM_SCGC5_LPTMR)
	nxp.LPTMR0.CSR.Set(nxp.LPTMR0_CSR_TIE)
	interrupt.New(nxp.IRQ_LPTMR0, wake).Enable()

	// 	analog_init();

	// #if !defined(TEENSY_INIT_USB_DELAY_BEFORE)
	// 	#if TEENSYDUINO >= 142
	// 		#define TEENSY_INIT_USB_DELAY_BEFORE 25
	// 	#else
	// 		#define TEENSY_INIT_USB_DELAY_BEFORE 50
	// 	#endif
	// #endif

	// #if !defined(TEENSY_INIT_USB_DELAY_AFTER)
	// 	#if TEENSYDUINO >= 142
	// 		#define TEENSY_INIT_USB_DELAY_AFTER 275
	// 	#else
	// 		#define TEENSY_INIT_USB_DELAY_AFTER 350
	// 	#endif
	// #endif

	// 	// for background about this startup delay, please see these conversations
	// 	// https://forum.pjrc.com/threads/36606-startup-time-(400ms)?p=113980&viewfull=1#post113980
	// 	// https://forum.pjrc.com/threads/31290-Teensey-3-2-Teensey-Loader-1-24-Issues?p=87273&viewfull=1#post87273

	// 	delay(TEENSY_INIT_USB_DELAY_BEFORE);
	// 	usb_init();
	// 	delay(TEENSY_INIT_USB_DELAY_AFTER);
}

func putchar(c byte) {
	u := &machine.UART1

	// ensure the UART has been configured
	if !u.SCGC.HasBits(u.SCGCMask) {
		u.Configure(machine.UARTConfig{})
	}

	for u.TCFIFO.Get() > 0 {
		// busy wait
	}
	u.D.Set(c)
}

// ???
const asyncScheduler = false

// convert from ticks (us) to time.Duration (ns)
const tickMicros = 1000

var cyclesPerMilli = machine.CPUFrequency() / 1000

// cyclesPerMilli-1 is used for the systick reset value
//   the systick current value will be decremented on every clock cycle
//   an interrupt is generated when the current value reaches 0
//   a value of freq/1000 generates a tick (irq) every millisecond (1/1000 s)

// number of systick irqs (milliseconds) since boot
var systickCount volatile.Register32

//go:export SysTick_Handler
func tick() {
	systickCount.Set(systickCount.Get() + 1)
}

type timeUnit int64

// ticks are in microseconds
func ticks() timeUnit {
	m := arm.DisableInterrupts()
	current := nxp.SysTick.CVR.Get()        // current value of the systick counter
	count := systickCount.Get()             // number of milliseconds since boot
	istatus := nxp.SystemControl.ICSR.Get() // interrupt status register
	arm.EnableInterrupts(m)

	micros := count * 1000 // a tick (1ms) = 1000 us

	// if the systick counter was about to reset and ICSR indicates a pending systick irq, increment count
	if istatus&nxp.SystemControl_ICSR_PENDSTSET != 0 && current > 50 {
		micros += 1000
	} else {
		cycles := cyclesPerMilli - 1 - current // number of cycles since last 1ms tick
		cyclesPerMicro := machine.CPUFrequency() / 1000000
		micros += cycles / cyclesPerMicro
	}

	return timeUnit(micros)
}

func wake(interrupt.Interrupt) {
	nxp.LPTMR0.CSR.Set(nxp.LPTMR0.CSR.Get()&^nxp.LPTMR0_CSR_TEN | nxp.LPTMR0_CSR_TCF) // clear flag and disable
}

// sleepTicks spins for a number of microseconds
func sleepTicks(duration timeUnit) {
	now := ticks()
	end := duration + now
	cyclesPerMicro := machine.ClockFrequency() / 1000000

	if duration <= 0 {
		return
	}

	nxp.LPTMR0.PSR.Set(nxp.LPTMR0_PSR_PCS(3) | nxp.LPTMR0_PSR_PBYP) // use 16MHz clock, undivided

	for now < end {
		count := uint32(end-now) / cyclesPerMicro
		if count > 65535 {
			count = 65535
		}

		nxp.LPTMR0.CMR.Set(count)                  // set count
		nxp.LPTMR0.CSR.SetBits(nxp.LPTMR0_CSR_TEN) // enable
		for nxp.LPTMR0.CSR.HasBits(nxp.LPTMR0_CSR_TEN) {
			arm.Asm("wfi")
		}

		now = ticks()
	}
}
