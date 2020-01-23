// +build nxp,mk66f18

package volatile

import "unsafe"

const registerBase = 0x40000000
const registerEnd = 0x40100000
const bitbandBase = 0x42000000
const ptrBytes = unsafe.Sizeof(uintptr(0))

//go:inline
func bitbandAddress(reg uintptr, bit uintptr) uintptr {
	if bit > ptrBytes*8 {
		panic("invalid bit position")
	}
	if reg < registerBase || reg >= registerEnd {
		panic("register is out of range")
	}
	return (reg-registerBase)*ptrBytes*8 + bit*ptrBytes + bitbandBase
}

// Bit maps bit N of register R to the corresponding bitband address. Bit panics
// if R is not an AIPS or GPIO register or if N is out of range (greater than
// the number of bits in a register minus one).
//
// go:inline
func (r *Register8) Bit(bit uintptr) *Register32 {
	ptr := bitbandAddress(uintptr(unsafe.Pointer(&r.Reg)), bit)
	return (*Register32)(unsafe.Pointer(ptr))
}

// Bit maps bit N of register R to the corresponding bitband address. Bit panics
// if R is not an AIPS or GPIO register or if N is out of range (greater than
// the number of bits in a register minus one).
//
// go:inline
func (r *Register16) Bit(bit uintptr) *Register32 {
	ptr := bitbandAddress(uintptr(unsafe.Pointer(&r.Reg)), bit)
	return (*Register32)(unsafe.Pointer(ptr))
}

// Bit maps bit N of register R to the corresponding bitband address. Bit panics
// if R is not an AIPS or GPIO register or if N is out of range (greater than
// the number of bits in a register minus one).
//
// go:inline
func (r *Register32) Bit(bit uintptr) *Register32 {
	ptr := bitbandAddress(uintptr(unsafe.Pointer(&r.Reg)), bit)
	return (*Register32)(unsafe.Pointer(ptr))
}
