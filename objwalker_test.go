package objwalker

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	testErr = errors.New("test")
)

func TestWalker_LoopProtected(t *testing.T) {
	type S struct {
		P *S
	}

	s := S{}
	s.P = &s

	t.Run("Protected", func(t *testing.T) {
		callTimes := 0
		err := New(func(info WalkInfo) error {
			callTimes++
			return nil
		}).Walk(&s)
		require.NoError(t, err)

		// call for:
		// 1. Ptr for original struct
		// 2. Original struct
		// 3. Pointer s.P
		// but not recursive call to s.P value - because it point to someself and visited already
		require.Equal(t, 3, callTimes)
	})
	t.Run("NoProtected", func(t *testing.T) {
		callTimes := 0
		callTimesLimit := 10
		err := New(func(info WalkInfo) error {
			callTimes++
			if callTimes == callTimesLimit {
				return testErr
			}
			return nil
		}).WithDisableLoopProtection().Walk(&s)
		require.Equal(t, testErr, err)
		require.Equal(t, callTimesLimit, callTimes)
	})
}

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

	t.Run("nil", func(t *testing.T) {
		called := false
		err := New(func(info WalkInfo) error {
			called = true
			return nil
		}).Walk(nil)
		require.NoError(t, err)
		require.False(t, called)
	})
}

func TestWalker_WalkArray(t *testing.T) {
	for _, testName := range []string{"Ok", "Skip", "ErrorArray", "ErrorItem"} {
		t.Run(testName, func(t *testing.T) {
			val := [2]int{1, 2}
			wasArray := false
			wasOne := false
			wasTwo := false
			err := New(func(info WalkInfo) error {
				if info.Value.Kind() == reflect.Array {
					wasArray = true
					if testName == "Skip" {
						return ErrSkip
					}
					if testName == "ErrorArray" {
						return testErr
					}
				}
				if info.Value.Kind() == reflect.Int {
					if info.Value.Interface().(int) == 1 {
						wasOne = true
					}
					if info.Value.Interface().(int) == 2 {
						wasTwo = true
					}
					if testName == "ErrorItem" {
						return testErr
					}
				}
				return nil
			}).Walk(val)

			switch testName {
			case "Ok":
				require.NoError(t, err)
				require.True(t, wasArray)
				require.True(t, wasOne)
				require.True(t, wasTwo)
			case "Skip":
				require.NoError(t, err)
				require.True(t, wasArray)
				require.False(t, wasOne)
				require.False(t, wasTwo)
			case "ErrorArray":
				require.ErrorIs(t, err, testErr)
				require.True(t, wasArray)
				require.False(t, wasOne)
				require.False(t, wasTwo)
			case "ErrorItem":
				require.ErrorIs(t, err, testErr)
				require.True(t, wasArray)
				require.True(t, wasOne)
				require.False(t, wasTwo)
			default:
				t.Fatal(testName)
			}
		})
	}
}

func TestWalker_WalkChan(t *testing.T) {
	val := make(chan bool)
	t.Run("ok", func(t *testing.T) {
		require.NoError(t, New(func(info WalkInfo) error {
			require.Equal(t, reflect.Chan, info.Value.Kind())
			require.NotZero(t, info.InternalStructSize)
			return nil
		}).Walk(val))
	})
	t.Run("Err", func(t *testing.T) {
		require.ErrorIs(t, New(func(info WalkInfo) error {
			return testErr
		}).Walk(val), testErr)
	})
}

func TestWalker_WalkFlat(t *testing.T) {
	val := 4
	t.Run("Ok", func(t *testing.T) {
		require.NoError(t, New(func(info WalkInfo) error {
			require.Equal(t, reflect.Int, info.Value.Kind())
			return nil
		}).Walk(val))
	})
	t.Run("Err", func(t *testing.T) {
		require.ErrorIs(t, New(func(info WalkInfo) error {
			return testErr
		}).Walk(val), testErr)
	})
}

func TestWalker_WalkFunc(t *testing.T) {
	val := func() int { return 1 }
	t.Run("Ok", func(t *testing.T) {
		require.NoError(t, New(func(info WalkInfo) error {
			require.Equal(t, reflect.Func, info.Value.Kind())
			return nil
		}).Walk(val))
	})

	t.Run("Err", func(t *testing.T) {
		require.ErrorIs(t, New(func(info WalkInfo) error {
			require.Equal(t, reflect.Func, info.Value.Kind())
			return testErr
		}).Walk(val), testErr)
	})
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

	t.Run("Nil", func(t *testing.T) {
		var m map[int]int
		callTimes := 0
		require.NoError(t, New(func(info WalkInfo) error {
			callTimes++
			return nil
		}).Walk(m))
	})

	for _, testName := range []string{"Ok", "SkipMap", "SkipKey", "ErrorMap", "ErrorKey", "ErrorValue"} {
		t.Run(testName, func(t *testing.T) {
			var val = map[int]string{1: "2"}
			wasMap := false
			wasKey := false
			wasValue := false

			err := New(func(info WalkInfo) error {
				if info.Value.Kind() == reflect.Map {
					wasMap = true
					if testName == "SkipMap" {
						return ErrSkip
					}
					if testName == "ErrorMap" {
						return testErr
					}
				}
				if info.Value.Kind() == reflect.Int {
					wasKey = true
					require.True(t, info.IsMapKey)
					require.Equal(t, info.Value.Int(), int64(1))
					if testName == "SkipKey" {
						return ErrSkip
					}
					if testName == "ErrorKey" {
						return testErr
					}
				}
				if info.Value.Kind() == reflect.String {
					wasValue = true
					require.True(t, info.IsMapValue)
					require.Equal(t, info.Value.String(), "2")
					if testName == "ErrorValue" {
						return testErr
					}
				}
				return nil
			}).Walk(val)
			require.True(t, wasMap)

			switch testName {
			case "Ok":
				require.NoError(t, err)
				require.True(t, wasKey)
				require.True(t, wasValue)
			case "SkipMap":
				require.NoError(t, err)
				require.False(t, wasKey)
				require.False(t, wasValue)
			case "SkipKey":
				require.NoError(t, err)
				require.True(t, wasKey)
				require.False(t, wasValue)
			case "ErrorMap":
				require.ErrorIs(t, err, testErr)
				require.False(t, wasKey)
				require.False(t, wasValue)
			case "ErrorKey":
				require.ErrorIs(t, err, testErr)
				require.True(t, wasKey)
				require.False(t, wasValue)
			case "ErrorValue":
				require.ErrorIs(t, err, testErr)
				require.True(t, wasKey)
				require.True(t, wasValue)
			default:
				t.Fatal(testName)
			}
		})
	}
}

func TestWalker_Ptr(t *testing.T) {
	t.Run("int", func(t *testing.T) {
		for _, testName := range []string{"Ok", "Skip", "Error"} {
			t.Run(testName, func(t *testing.T) {
				vInt := 2
				var val = &vInt
				wasPtr := false
				wasInt := false
				err := New(func(info WalkInfo) error {
					if info.Value.Kind() == reflect.Ptr {
						wasPtr = true
						if testName == "Skip" {
							return ErrSkip
						}
						if testName == "Error" {
							return testErr
						}
					}
					if info.Value.Kind() == reflect.Int {
						wasInt = true
						require.Equal(t, info.Value.Int(), int64(2))
					}
					return nil
				}).Walk(val)
				switch testName {
				case "Ok":
					require.NoError(t, err)
					require.True(t, wasPtr)
					require.True(t, wasInt)
				case "Skip":
					require.NoError(t, err)
					require.True(t, wasPtr)
					require.False(t, wasInt)
				case "Error":
					require.ErrorIs(t, err, testErr)
					require.True(t, wasPtr)
					require.False(t, wasInt)
				default:
					t.Fatal(testName)
				}

			})
		}
	})
	t.Run("nil", func(t *testing.T) {
		var val *int
		require.NoError(t, New(func(info WalkInfo) error {
			return nil
		}).Walk(val))

	})
}

func TestWalker_WalkValue(t *testing.T) {
	t.Run("BadKind", func(t *testing.T) {
		walker := New(func(info WalkInfo) error {
			return nil
		})
		err := walker.walkValue(WalkInfo{})
		require.ErrorIs(t, err, ErrUnknownKind)
	})
}

func TestWalker_WalkSlice(t *testing.T) {
	for _, testName := range []string{"Ok", "Skip", "Error", "ErrorItem"} {
		t.Run(testName, func(t *testing.T) {
			val := []int{1, 2}
			wasSlice := false
			wasOne := false
			wasTwo := false
			err := New(func(info WalkInfo) error {
				if info.Value.Kind() == reflect.Slice {
					wasSlice = true
					if testName == "Skip" {
						return ErrSkip
					}
					if testName == "Error" {
						return testErr
					}
				}
				if info.Value.Kind() == reflect.Int {
					if info.Value.Interface().(int) == 1 {
						wasOne = true
						if testName == "ErrorItem" {
							return testErr
						}
					}
					if info.Value.Interface().(int) == 2 {
						wasTwo = true
					}
				}
				return nil
			}).Walk(val)

			switch testName {
			case "Ok":
				require.NoError(t, err)
				require.True(t, wasSlice)
				require.True(t, wasOne)
				require.True(t, wasTwo)
			case "Skip":
				require.NoError(t, err)
				require.True(t, wasSlice)
				require.False(t, wasOne)
				require.False(t, wasTwo)
			case "Error":
				require.ErrorIs(t, err, testErr)
				require.True(t, wasSlice)
				require.False(t, wasOne)
				require.False(t, wasTwo)
			case "ErrorItem":
				require.ErrorIs(t, err, testErr)
				require.True(t, wasSlice)
				require.True(t, wasOne)
				require.False(t, wasTwo)
			default:
				t.Fatal(testName)
			}

		})
	}
}

func TestWalkString(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		val := ""
		require.NoError(t, New(func(info WalkInfo) error {
			require.Equal(t, reflect.String, info.Value.Kind())
			require.NotZero(t, info.InternalStructSize)
			return nil
		}).Walk(val))
	})
	t.Run("str", func(t *testing.T) {
		val := "str"
		require.NoError(t, New(func(info WalkInfo) error {
			if info.Value.Kind() == reflect.String {
				require.Equal(t, reflect.String, info.Value.Kind())
				require.NotZero(t, info.InternalStructSize)
				require.True(t, info.HasDirectPointer())
				require.True(t, info.HasDataPointer())
			}
			return nil
		}).Walk(&val))
	})
}

func TestWalkStruct(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		val := struct{}{}
		require.NoError(t, New(func(info WalkInfo) error {
			require.Equal(t, reflect.Struct, info.Value.Kind())
			return nil
		}).Walk(val))
	})
	t.Run("Fields", func(t *testing.T) {
		val := struct {
			Pub  int
			priv string
		}{}

		for _, testName := range []string{"Ok", "Skip", "Error"} {
			t.Run(testName, func(t *testing.T) {
				wasStruct := false
				wasPublic := false
				wasPrivate := false
				err := New(func(info WalkInfo) error {
					kind := info.Value.Kind()
					if kind == reflect.Struct {
						wasStruct = true
						if testName == "Skip" {
							return ErrSkip
						}
						if testName == "Error" {
							return testErr
						}
					}
					if kind == reflect.Int {
						wasPublic = true
					}
					if kind == reflect.String {
						wasPrivate = true
					}
					if kind != reflect.Ptr {
						require.NotZero(t, info.DirectPointer)
					}
					return nil
				}).Walk(&val)

				switch testName {
				case "Ok":
					require.NoError(t, err)
					require.True(t, wasStruct)
					require.True(t, wasPublic)
					require.True(t, wasPrivate)
				case "Skip":
					require.NoError(t, err)
					require.True(t, wasStruct)
					require.False(t, wasPublic)
					require.False(t, wasPrivate)
				case "Error":
					require.ErrorIs(t, err, testErr)
					require.True(t, wasStruct)
					require.False(t, wasPublic)
					require.False(t, wasPrivate)
				default:
					t.Fatal(testName)
				}
			})

		}
	})
}

func ExampleWalker() {
	type S struct {
		Val1  int
		Slice []string
	}

	v := S{
		Val1:  2,
		Slice: []string{"hello", "world"},
	}
	_ = New(func(info WalkInfo) error {
		val := info.Value.Interface()
		_ = val
		if info.IsStructField {
			fmt.Println(info.Value.Interface())
		}
		return nil
	}).Walk(v)

	// Output:
	// 2
	// [hello world]
}
