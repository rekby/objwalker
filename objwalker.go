package objwalker

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"unsafe"
)

var zeroPointer unsafe.Pointer

var (
	// ErrSkip - signal for skip iteration over value
	// can be returned for array, interface, map, map key, slice, struct, ptr,
	// for other kinds - unspecified behaviour and it may be change for feature versions
	ErrSkip = errors.New("skip value")

	// ErrUnknownKind mean reflect walk see unknown kind of type - need to update library
	ErrUnknownKind = errors.New("unknown kind")
	ErrInvalidKind = errors.New("invalid kind")
)

// WalkInfo send to walk callback with every value
type WalkInfo struct {
	// Value - reflection Value for inspect/manupulate variable
	Value reflect.Value

	// IsFlat mean Value is simple builtin value
	IsFlat bool

	// IsMapKey mean Value direct use in map key
	IsMapKey bool

	// IsMapValue mean Value direct used in map value
	IsMapValue bool

	// IsStructField mean Value is direct used as struct field
	IsStructField bool

	// IsVisited true if loop protection disabled and walker detect about value was visited already
	IsVisited bool

	// InternalStructSize fixed size of internal struct, not contained content size
	// approximately - from go version and may be stale
	InternalStructSize int

	// DirectPointer hold address of Value data (Value.ptr) 0 if value not addresable
	DirectPointer unsafe.Pointer

	// Pointer to underly data buffer of string and slice if available
	DataPointer unsafe.Pointer
}

// HasDirectPointer check if w.DirectPointer has non zero value
func (w *WalkInfo) HasDirectPointer() bool {
	return w.DirectPointer != zeroPointer
}

// HasDataPointer check if w.DataPointer has non zero value
func (w *WalkInfo) HasDataPointer() bool {
	return w.DataPointer != zeroPointer
}

func newWalkerInfo(v reflect.Value) (res WalkInfo) {
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
	visited  map[unsafe.Pointer]map[reflect.Type]empty

	//nolint:unused,structcheck
	_denyCopyByValue sync.Mutex // error in go vet if try to copy Walker by value
}

func (walker *Walker) WithDisableLoopProtection() *Walker {
	walker.LoopProtection = false
	return walker
}

// New create new walker with f callback
// f will call for every field, item, etc of walked object
// f can called multiply times for same address with different item type
// for example:
// type T struct { Val int }
// f will called for struct T and for Pub int
func New(f WalkFunc) *Walker {
	return &Walker{
		LoopProtection: true,

		callback: f,
		visited:  make(map[unsafe.Pointer]map[reflect.Type]empty),
	}
}

// Walk - deep walk by v with reflection
// v must be pointer
func (walker *Walker) Walk(v interface{}) error {
	if v == nil {
		return nil
	}

	valueInfo := newWalkerInfo(reflect.ValueOf(v))
	return walker.walkValue(valueInfo)
}

func (walker *Walker) walkValue(info WalkInfo) error {
	if info.DirectPointer != zeroPointer {
		types := walker.visited[info.DirectPointer]
		if types == nil {
			types = make(map[reflect.Type]empty)
			walker.visited[info.DirectPointer] = types
		}

		t := info.Value.Type()
		_, okType := types[t]
		if okType {
			info.IsVisited = true
		} else {
			types[t] = empty{}
		}

		if info.IsVisited && walker.LoopProtection {
			return nil
		}
	}

	switch info.Value.Kind() {
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
		return fmt.Errorf("can't walk into kind %v value: %w", info.Value.Kind(), ErrUnknownKind)
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
		if err := walker.walkValue(iteminfo); err != nil {
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
	const sliceSize = int(unsafe.Sizeof(reflect.SliceHeader{}) + uintptr(-int(unsafe.Sizeof(reflect.SliceHeader{}))&(maxAlign-1)))

	info.InternalStructSize = sliceSize
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
	const stringHeaderSize = int(unsafe.Sizeof(reflect.StringHeader{}) + uintptr(-int(unsafe.Sizeof(reflect.StringHeader{}))&(maxAlign-1)))

	info.InternalStructSize = stringHeaderSize
	if info.Value.Len() > 0 {
		s := info.Value.String()
		sPointer := &s
		sInternalPointer := (*reflect.StringHeader)(unsafe.Pointer(sPointer))
		info.DataPointer = unsafe.Pointer(sInternalPointer.Data)
	}

	return walker.callback(info)
}

func (walker *Walker) walkStruct(info WalkInfo) error {
	if err := walker.callback(info); err != nil {
		if errors.Is(err, ErrSkip) {
			return nil
		}
		return err
	}

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
