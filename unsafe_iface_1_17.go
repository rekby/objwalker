package objwalker

import "unsafe"

const ifaceSize = unsafe.Sizeof(iface{}) + uintptr(-int(unsafe.Sizeof(iface{}))&(maxAlign-1))

func interfaceSize() int {
	return int(ifaceSize)
}

// copy from https://github.com/golang/go/blob/9de1ac6ac2cad3871760d0aa288f5ca713afd0a6/src/runtime/runtime2.go#L202
type iface struct {
	tab  *itab
	data unsafe.Pointer
}

// stub - for calc pointer size only
type itab int
