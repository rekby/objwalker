package objwalker

import (
	"reflect"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
)

func TestValue(t *testing.T) {
	require.True(t, checkValue())

	var iVal int
	rVal := reflect.ValueOf(&iVal)
	reflectOfReflect := reflect.ValueOf(&rVal).Elem()
	ptrField := reflectOfReflect.FieldByName("ptr")
	ptrFieldAddr := ptrField.UnsafeAddr()

	internalValue := newValue(&rVal)
	internalPointer := unsafe.Pointer(&internalValue.ptr)
	require.Equal(t, ptrFieldAddr, uintptr(internalPointer))
}
