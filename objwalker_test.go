package objwalker

import (
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWalker_Walk(t *testing.T) {
	t.Run("Ok", func(t *testing.T) {
		walker := New(func(info WalkInfo) error {
			return nil
		})
		var v int
		err := walker.Walk(v)
		require.NoError(t, err)
	})

	t.Run("Deep", func(t *testing.T) {
		type TestStructA struct {
			Val int
		}
		type TestStructB struct {
			Val1 int
			Test TestStructA
		}

		var v TestStructB
		err := New(func(info WalkInfo) error {
			kind := info.Value.Kind()
			if kind == reflect.Int {
				info.Value.SetInt(1)
			}
			return nil
		}).Walk(&v)
		require.NoError(t, err)
		require.Equal(t, TestStructB{
			Val1: 1,
			Test: TestStructA{
				Val: 1,
			},
		}, v)
	})

	t.Run("ChangePrivateField", func(t *testing.T) {
		type TestStruct struct {
			f int
		}
		t.Run("UsualReflection", func(t *testing.T) {
			var v TestStruct
			require.Panics(t, func() {
				_ = New(func(info WalkInfo) error {
					if info.Value.Kind() == reflect.Int {
						info.Value.SetInt(1)
					}
					return nil
				}).Walk(&v)
			})
		})
		t.Run("DirectPointer", func(t *testing.T) {
			// change private field by reflection is denied, but it is possible through direct pointer
			// usually it is bad idea
			var v TestStruct
			err := New(func(info WalkInfo) error {
				if info.Value.Kind() == reflect.Int {
					val := reflect.NewAt(info.Value.Type(), info.DirectPointer)
					val.Elem().SetInt(1)
				}
				return nil
			}).Walk(&v)
			require.NoError(t, err)
			require.Equal(t, TestStruct{1}, v)
		})
	})
}

func TestWalker_WalkArray(t *testing.T) {
	val := [2]int{1, 2}
	wasArray := false
	wasOne := false
	wasTwo := false
	require.NoError(t, New(func(info WalkInfo) error {
		if info.Value.Kind() == reflect.Array {
			wasArray = true
		}
		if info.Value.Kind() == reflect.Int {
			if info.Value.Interface().(int) == 1 {
				wasOne = true
			}
			if info.Value.Interface().(int) == 2 {
				wasTwo = true
			}
		}
		return nil
	}).Walk(val))
	require.True(t, wasArray)
	require.True(t, wasOne)
	require.True(t, wasTwo)
}

func TestWalker_WalkChan(t *testing.T) {
	val := make(chan bool)
	require.NoError(t, New(func(info WalkInfo) error {
		require.Equal(t, reflect.Chan, info.Value.Kind())
		require.NotZero(t, info.InternalStructSize)
		return nil
	}).Walk(val))
}

func TestWalker_WalkFlat(t *testing.T) {
	val := 4
	require.NoError(t, New(func(info WalkInfo) error {
		require.Equal(t, reflect.Int, info.Value.Kind())
		return nil
	}).Walk(val))
}

func TestWalker_WalkFunc(t *testing.T) {
	val := func() int { return 1 }
	require.NoError(t, New(func(info WalkInfo) error {
		require.Equal(t, reflect.Func, info.Value.Kind())
		return nil
	}).Walk(val))
}

func TestWalker_Interface(t *testing.T) {
	testErr := errors.New("asd")
	val := []error{testErr}
	wasInterface := false
	require.NoError(t, New(func(info WalkInfo) error {
		if info.Value.Kind() == reflect.Interface {
			wasInterface = true
			require.Equal(t, testErr, info.Value.Interface())
		}
		return nil
	}).Walk(val))
	require.True(t, wasInterface)
}

func TestWalker_Map(t *testing.T) {
	var val = map[int]string{1: "2"}
	wasMap := false
	wasKey := true
	wasValue := true
	require.NoError(t, New(func(info WalkInfo) error {
		if info.Value.Kind() == reflect.Map {
			wasMap = true
		}
		if info.Value.Kind() == reflect.Int {
			wasKey = true
			require.True(t, info.IsMapKey)
			require.Equal(t, info.Value.Int(), int64(1))
		}
		if info.Value.Kind() == reflect.String {
			wasValue = true
			require.True(t, info.IsMapValue)
			require.Equal(t, info.Value.String(), "2")

		}
		return nil
	}).Walk(val))
	require.True(t, wasMap)
	require.True(t, wasKey)
	require.True(t, wasValue)
}

func TestWalker_Ptr(t *testing.T) {
	t.Run("int", func(t *testing.T) {

		vInt := 2
		var val = &vInt
		wasPtr := false
		wasInt := false
		require.NoError(t, New(func(info WalkInfo) error {
			if info.Value.Kind() == reflect.Ptr {
				wasPtr = true
			}
			if info.Value.Kind() == reflect.Int {
				wasInt = true
				require.Equal(t, info.Value.Int(), int64(2))
			}
			return nil
		}).Walk(val))
		require.True(t, wasPtr)
		require.True(t, wasInt)
	})
	t.Run("nil", func(t *testing.T) {
		var val *int
		require.NoError(t, New(func(info WalkInfo) error {
			return nil
		}).Walk(val))

	})
}

func TestWalker_WalkSlice(t *testing.T) {
	val := []int{1, 2}
	wasSlice := false
	wasOne := false
	wasTwo := false
	require.NoError(t, New(func(info WalkInfo) error {
		if info.Value.Kind() == reflect.Slice {
			wasSlice = true
		}
		if info.Value.Kind() == reflect.Int {
			if info.Value.Interface().(int) == 1 {
				wasOne = true
			}
			if info.Value.Interface().(int) == 2 {
				wasTwo = true
			}
		}
		return nil
	}).Walk(val))
	require.True(t, wasSlice)
	require.True(t, wasOne)
	require.True(t, wasTwo)
}
