package objwalker

import "unsafe"

const stringStructSize = unsafe.Sizeof(stringStruct{}) + uintptr(-int(unsafe.Sizeof(stringStruct{}))&(maxAlign-1))

func stringSize() int {
	return int(stringStructSize)
}

// copy from https://github.com/golang/go/blob/9de1ac6ac2cad3871760d0aa288f5ca713afd0a6/src/runtime/string.go#L228
type stringStruct struct {
	str unsafe.Pointer
	len int
}
