// +build nxp,mk66f18

package machine

type PinMode uint32

const (
	peripheralTypeMask   = 0x0000FFFF
	peripheralSignalMask = 0x00FF0000
	// peripheralNumberMask = 0xFF000000

	peripheralTypeOffset   = 0
	peripheralSignalOffset = 16
	// peripheralNumberOffset = 24
)

const (
	PinDisable PinMode = iota << peripheralTypeOffset

	// analog to digital converter
	PinADC

	// controller area network
	PinCAN

	// analog comparator
	PinCMP

	// carrier modulator transmitter
	PinCMT

	// digital I/O
	PinDIO

	// IEEE 1588 timers (ethernet)
	PinENET

	// flexible timer module
	PinFTM

	// inter-integrated circuit
	PinI2C

	// inter-IC sound
	PinI2S

	// low power timer
	PinLPTMR

	// low power universal asynchronous receiver/transmitter
	PinLPUART

	// media independent interface (ethernet)
	PinMII

	// reduced media independent interface (ethernet)
	PinRMII

	// secure digital high capacity (SD card)
	PinSDHC

	// serial peripheral interface
	PinSPI

	// timer/PWM module
	PinTPM

	// touch sensing interface
	PinTSI

	// universal asynchronous receiver/transmitter
	PinUART
)

const (
	PinInput PinMode = (iota+1)<<peripheralSignalOffset | PinDIO
	PinInputPullUp
	PinInputPullDown
	PinOutput
	PinOutputOpenDrain
)

const (
	ftmPinCH0 PinMode = (iota+1)<<peripheralSignalOffset | PinFTM
	ftmPinCH1
	ftmPinCH2
	ftmPinCH3
	ftmPinCH4
	ftmPinCH5
	ftmPinCH6
	ftmPinCH7
	ftmPinCLKIN0
	ftmPinCLKIN1
	ftmPinFLT0
	ftmPinFLT1
	ftmPinFLT2
	ftmPinFLT3
	ftmPinQD_PHA
	ftmPinQD_PHB
)

const (
	i2cPinSCL PinMode = (iota+1)<<peripheralSignalOffset | PinI2C
	i2cPinSDA
)

const (
	i2sPinMCLK PinMode = (iota+1)<<peripheralSignalOffset | PinI2S
	i2sPinRDX0
	i2sPinRDX1
	i2sPinRX_BCLK
	i2sPinRX_FS
	i2sPinRXD0
	i2sPinRXD1
	i2sPinTX_BCLK
	i2sPinTX_FS
	i2sPinTX0
	i2sPinTXD0
	i2sPinTXD1
)

const (
	lptmrPinALT1 PinMode = (iota+1)<<peripheralSignalOffset | PinLPTMR
	lptmrPinALT2
)

const (
	lpuartPinCTS PinMode = (iota+1)<<peripheralSignalOffset | PinLPUART
	lpuartPinRTS
	lpuartPinRX
	lpuartPinTX
)

const (
	miiPinCOL PinMode = (iota+1)<<peripheralSignalOffset | PinMII
	miiPinCRS
	miiPinMDC
	miiPinMDIO
	miiPinRXCLK
	miiPinRXD0
	miiPinRXD1
	miiPinRXD2
	miiPinRXD3
	miiPinRXDV
	miiPinRXER
	miiPinTXCLK
	miiPinTXD0
	miiPinTXD1
	miiPinTXD2
	miiPinTXD3
	miiPinTXEN
	miiPinTXER
)

const (
	rmiiPinCRS_DV PinMode = (iota+1)<<peripheralSignalOffset | PinRMII
	rmiiPinMDC
	rmiiPinMDIO
	rmiiPinRXD0
	rmiiPinRXD1
	rmiiPinRXER
	rmiiPinTXD0
	rmiiPinTXD1
	rmiiPinTXEN
)

const (
	sdhcPinCLKIN PinMode = (iota+1)<<peripheralSignalOffset | PinSDHC
	sdhcPinCMD
	sdhcPinD0
	sdhcPinD1
	sdhcPinD2
	sdhcPinD3
	sdhcPinD4
	sdhcPinD5
	sdhcPinD6
	sdhcPinD7
	sdhcPinDCLK
)

const (
	spiPinPCS0 PinMode = (iota+1)<<peripheralSignalOffset | PinSPI
	spiPinPCS1
	spiPinPCS2
	spiPinPCS3
	spiPinPCS4
	spiPinPCS5
	spiPinSCK
	spiPinSIN
	spiPinSOUT
)

const (
	tpmPinCH0 PinMode = (iota+1)<<peripheralSignalOffset | PinTPM
	tpmPinCH1
	tpmPinCLKIN0
	tpmPinCLKIN1
)

const (
	uartPinCTS PinMode = (iota+1)<<peripheralSignalOffset | PinUART
	uartPinRTS
	uartPinRX
	uartPinTX
)

const (
	PA00 Pin = iota
	PA01
	PA02
	PA03
	PA04
	PA05
	PA06
	PA07
	PA08
	PA09
	PA10
	PA11
	PA12
	PA13
	PA14
	PA15
	PA16
	PA17
	PA18
	PA19
	PA20
	PA21
	PA22
	PA23
	PA24
	PA25
	PA26
	PA27
	PA28
	PA29
)

const (
	PB00 Pin = iota + 32
	PB01
	PB02
	PB03
	PB04
	PB05
	PB06
	PB07
	PB08
	PB09
	PB10
	PB11
	_
	_
	_
	_
	PB16
	PB17
	PB18
	PB19
	PB20
	PB21
	PB22
	PB23
)

const (
	PC00 Pin = iota + 64
	PC01
	PC02
	PC03
	PC04
	PC05
	PC06
	PC07
	PC08
	PC09
	PC10
	PC11
	PC12
	PC13
	PC14
	PC15
	PC16
	PC17
	PC18
	PC19
)

const (
	PD00 Pin = iota + 96
	PD01
	PD02
	PD03
	PD04
	PD05
	PD06
	PD07
	PD08
	PD09
	PD10
	PD11
	PD12
	PD13
	PD14
	PD15
)

const (
	PE00 Pin = iota + 128
	PE01
	PE02
	PE03
	PE04
	PE05
	PE06
	PE07
	PE08
	PE09
	PE10
	PE11
	PE12
	PE13
	PE14
	PE15
	PE16
	PE17
	PE18
	PE19
	PE20
	PE21
	PE22
	PE23
	PE24
	PE25
	PE26
	PE27
	PE28
)
