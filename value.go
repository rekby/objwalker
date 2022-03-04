package objwalker

import (
	"reflect"
	"unsafe"
)

// value repeat struct of reflect.Value
// ptr field is only need for the package
type value struct {
	_ unsafe.Pointer

	// Pointer-valued data or, if flagIndir is set, pointer to data.
	// Valid when either flagIndir is set or typ.pointers() is true.
	ptr unsafe.Pointer

	// rest
}

func newValue(r *reflect.Value) *value {
	unsafePointer := (unsafe.Pointer)(r)
	return (*value)(unsafePointer)
}

func checkValue() bool {
	var iVal int
	rVal := reflect.ValueOf(&iVal)
	internalValue := newValue(&rVal)

	iValPtr := (unsafe.Pointer)(&iVal)
	return iValPtr == internalValue.ptr
}
