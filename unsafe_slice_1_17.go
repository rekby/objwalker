package objwalker

import "unsafe"

const (
	sliceStructSize = unsafe.Sizeof(slice{}) + uintptr(-int(unsafe.Sizeof(slice{}))&(maxAlign-1))
)

func sliceSize() int {
	return int(sliceStructSize)
}

// copy from https://github.com/golang/go/blob/9de1ac6ac2cad3871760d0aa288f5ca713afd0a6/src/runtime/slice.go

type slice struct {
	array unsafe.Pointer
	len   int
	cap   int
}
