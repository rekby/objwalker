package objwalker

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"unsafe"
)

// InternalsMaxGoVersion - go version from which actualize internal struct sizes
const InternalsMaxGoVersion = "go1.17"

var zeroPointer unsafe.Pointer

var (
	// ErrSkip - signal for skip iteration over value
	// can be returned for map, map key, array, slice, ptr, interface
	// for other kinds - unspecified behaviour and it may be change for feature versions
	ErrSkip = errors.New("skip value")

	// ErrUnknownKind mean reflect walk see unknown kind of type - need to update library
	ErrUnknownKind = errors.New("unknown kind")
	ErrInvalidKind = errors.New("invalid kind")
)

type WalkInfo struct {
	Value reflect.Value

	IsFlat        bool
	IsMapKey      bool
	IsMapValue    bool
	IsStructField bool

	// IsVisited true if loop protection disabled and walker detect about value was visited already
	IsVisited bool

	// Fixed size of internal struct, not contained content size
	// approximately - from go version and may be stale
	InternalStructSize int

	// DirectPointer 0 if value not addresable
	DirectPointer unsafe.Pointer

	// Pointer to data of string, slice if available
	DataPointer unsafe.Pointer
}

func newWalkerInfo(v reflect.Value) (res WalkInfo) {
	runtime.Version()
	if v.CanAddr() {
		res.DirectPointer = unsafe.Pointer(v.UnsafeAddr())
	}
	res.Value = v
	return res
}

type WalkFunc func(info WalkInfo) error

type empty struct{}
type Walker struct {
	LoopProtection bool

	callback WalkFunc
	visited  map[unsafe.Pointer]empty
}

func New(f WalkFunc) *Walker {
	return &Walker{
		LoopProtection: true,

		callback: f,
		visited:  make(map[unsafe.Pointer]empty),
	}
}

// Walk - deep walk by v with reflection
// v must be pointer
func (walker *Walker) Walk(v interface{}) error {
	if v == nil {
		return nil
	}

	valueInfo := newWalkerInfo(reflect.ValueOf(v))
	err := walker.walkValue(valueInfo)
	if errors.Is(err, ErrSkip) {
		return nil
	}
	return err
}

func (walker *Walker) walkValue(info WalkInfo) error {
	if info.DirectPointer != zeroPointer {
		_, ok := walker.visited[info.DirectPointer]

		if ok {
			info.IsVisited = true
		} else {
			// not mark array and struct - for prevent false positive detection of them first item/field
			kind := info.Value.Kind()
			if kind != reflect.Array && kind != reflect.Struct {
				walker.visited[info.DirectPointer] = empty{}
			}
		}

		if info.IsVisited && walker.LoopProtection {
			return nil
		}
	}

	switch info.Value.Kind() {
	case reflect.Invalid:
		return fmt.Errorf("stop walking: %w", ErrInvalidKind)
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8,
		reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr, reflect.Float32, reflect.Float64, reflect.Complex64,
		reflect.Complex128, reflect.UnsafePointer:
		return walker.walkFlat(info)
	case reflect.Array:
		return walker.walkArray(info)
	case reflect.Chan:
		return walker.walkChan(info)
	case reflect.Func:
		return walker.walkFunc(info)
	case reflect.Interface:
		return walker.walkInterface(info)
	case reflect.Ptr:
		return walker.walkPtr(info)
	case reflect.Map:
		return walker.walkMap(info)
	case reflect.Slice:
		return walker.walkSlice(info)
	case reflect.String:
		return walker.walkString(info)
	case reflect.Struct:
		return walker.walkStruct(info)
	default:
		return fmt.Errorf("can't walk into type %v: %w", info.Value.Type(), ErrUnknownKind)
	}
}

func (walker *Walker) walkFlat(info WalkInfo) error {
	info.IsFlat = true
	return walker.callback(info)
}

func (walker *Walker) walkArray(info WalkInfo) error {
	if err := walker.callback(info); err != nil {
		if errors.Is(err, ErrSkip) {
			return nil
		}
		return err
	}

	vLen := info.Value.Len()
	for i := 0; i < vLen; i++ {
		item := info.Value.Index(i)
		iteminfo := newWalkerInfo(item)
		if err := walker.walkValue(iteminfo); err != nil && !errors.Is(err, ErrSkip) {
			return err
		}
	}
	return nil
}

func (walker *Walker) walkChan(info WalkInfo) error {
	info.InternalStructSize = chanStructSize()
	return walker.callback(info)
}

func (walker *Walker) walkFunc(info WalkInfo) error {
	return walker.callback(info)
}

func (walker *Walker) walkInterface(info WalkInfo) error {
	info.InternalStructSize = interfaceSize()
	return walker.walkPtr(info)
}

func (walker *Walker) walkPtr(info WalkInfo) error {
	if err := walker.callback(info); err != nil {
		if errors.Is(err, ErrSkip) {
			return nil
		}
		return err
	}
	if info.Value.IsNil() {
		return nil
	}
	elem := info.Value.Elem()
	return walker.walkValue(newWalkerInfo(elem))
}

func (walker *Walker) walkMap(info WalkInfo) error {
	info.InternalStructSize = mapSize()
	if err := walker.callback(info); err != nil {
		if errors.Is(err, ErrSkip) {
			return nil
		}
		return err
	}

	if info.Value.IsNil() {
		return nil
	}

	iterator := info.Value.MapRange()
	for iterator.Next() {
		key := iterator.Key()
		keyInfo := newWalkerInfo(key)
		keyInfo.IsMapKey = true

		if err := walker.walkValue(keyInfo); err != nil {
			if errors.Is(err, ErrSkip) {
				continue
			}
			return err
		}

		val := iterator.Value()
		valInfo := newWalkerInfo(val)
		valInfo.IsMapValue = true
		if err := walker.walkValue(valInfo); err != nil {
			return err
		}
	}
	return nil
}

func (walker *Walker) walkSlice(info WalkInfo) error {
	info.InternalStructSize = sliceSize()
	if info.Value.Len() > 0 {
		info.DataPointer = newWalkerInfo(info.Value.Index(0)).DirectPointer
	}

	if err := walker.callback(info); err != nil {
		if errors.Is(err, ErrSkip) {
			return nil
		}
		return err
	}

	sliceLen := info.Value.Len()
	for i := 0; i < sliceLen; i++ {
		item := info.Value.Index(i)
		if err := walker.walkValue(newWalkerInfo(item)); err != nil {
			return err
		}
	}

	return nil
}

func (walker *Walker) walkString(info WalkInfo) error {
	info.InternalStructSize = stringSize()
	if info.Value.Len() > 0 {
		info.DataPointer = newWalkerInfo(info.Value.Index(0)).DirectPointer
	}

	return walker.callback(info)
}

func (walker *Walker) walkStruct(info WalkInfo) error {
	numField := info.Value.NumField()
	for i := 0; i < numField; i++ {
		fieldVal := info.Value.Field(i)
		fieldInfo := newWalkerInfo(fieldVal)
		fieldInfo.IsStructField = true
		if err := walker.walkValue(fieldInfo); err != nil {
			return err
		}
	}
	return nil
}
