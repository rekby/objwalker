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

	// ErrInvalidKind
	errInvalidKind = errors.New("unexpected invalid kind")

	// ErrUnknownKind mean reflect walk see unknown kind of type - need to update library
	ErrUnknownKind = errors.New("unknown kind")
)

// WalkInfo send to walk callback with every value
type WalkInfo struct {
	// Value - reflection Value for inspect/manupulate variable
	Value reflect.Value

	// Parent is info of prev node in travel tree hierarchy
	// Parent == nil for first visited value
	Parent *WalkInfo

	// DirectPointer hold address of Value data (Value.ptr) 0 if value not addresable
	DirectPointer unsafe.Pointer

	// IsVisited true if loop protection disabled and walker detect about value was visited already
	IsVisited bool

	isMapValue bool
	isMapKey   bool
}

// HasDirectPointer check if w.DirectPointer has non zero value
func (w *WalkInfo) HasDirectPointer() bool {
	return w.DirectPointer != zeroPointer
}

// IsMapKey mean Value direct use as map key
func (w *WalkInfo) IsMapKey() bool {
	return w.isMapKey
}

// IsMapValue mean Value direct use as map value
func (w *WalkInfo) IsMapValue() bool {
	return w.isMapValue
}

func newWalkerInfo(v reflect.Value, parent *WalkInfo) *WalkInfo {
	var res WalkInfo
	if v.CanAddr() {
		res.DirectPointer = unsafe.Pointer(v.UnsafeAddr())
	}
	res.Value = v
	res.Parent = parent
	return &res
}

// WalkFunc is type of callback function
type WalkFunc func(info *WalkInfo) error

type empty struct{}

// Walker provide settings and state for Walk function
type Walker struct {
	LoopProtection bool
	callback       WalkFunc
}

// New create new walker with f callback
// f will call for every field, item, etc of walked object
// f can called multiply times for same address with different item type
// for example:
// type T struct { Val int }
// f will called for struct T and for Pub int
//
// if f return ErrSkip - skip the struct (, map, slice, ... see ErrSkip comment)
// if f return other non nil error - stop walk and return the error to walk caller
func New(f WalkFunc) *Walker {
	return &Walker{
		LoopProtection: true,
		callback:       f,
	}
}

// Walk create new walker with empty state and run Walk over object
func (w Walker) Walk(v interface{}) error {
	walker := newWalkerState(w)
	return walker.walk(v)
}

// WithDisableLoopProtection disable loop protection.
// callback must self-detect loops and return ErrSkip
func (w *Walker) WithDisableLoopProtection() *Walker {
	w.LoopProtection = false
	return w
}

type walkerState struct {
	Walker
	visited map[unsafe.Pointer]map[reflect.Type]empty

	//nolint:unused,structcheck
	_denyCopyByValue sync.Mutex // error in go vet if try to copy walkerState by value
}

func newWalkerState(opts Walker) *walkerState {
	return &walkerState{
		Walker:           opts,
		visited:          make(map[unsafe.Pointer]map[reflect.Type]empty),
		_denyCopyByValue: sync.Mutex{},
	}
}

func (state *walkerState) walk(v interface{}) error {
	if v == nil {
		return nil
	}

	valueInfo := newWalkerInfo(reflect.ValueOf(v), nil)
	return state.walkValue(valueInfo)
}

func (state *walkerState) loopDetector(info *WalkInfo) {
	if info.DirectPointer != zeroPointer {
		types := state.visited[info.DirectPointer]
		if types == nil {
			types = make(map[reflect.Type]empty)
			state.visited[info.DirectPointer] = types
		}

		t := info.Value.Type()
		_, okType := types[t]
		if okType {
			info.IsVisited = true
		} else {
			types[t] = empty{}
		}

	}
}

func (state *walkerState) walkValue(info *WalkInfo) error {
	state.loopDetector(info)
	if info.IsVisited && state.LoopProtection {
		return nil
	}

	return state.kindRoute(info.Value.Kind(), info)
}

func (state *walkerState) kindRoute(kind reflect.Kind, info *WalkInfo) error {
	switch kind {
	case reflect.Invalid:
		return errInvalidKind
	case reflect.Array:
		return state.walkArray(info)
	case reflect.Interface, reflect.Ptr:
		return state.walkPtr(info)
	case reflect.Map:
		return state.walkMap(info)
	case reflect.Slice:
		return state.walkSlice(info)
	case reflect.Chan, reflect.Func, reflect.String, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8,
		reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr, reflect.Float32, reflect.Float64, reflect.Complex64,
		reflect.Complex128, reflect.UnsafePointer:
		return state.walkSimple(info)
	case reflect.Struct:
		return state.walkStruct(info)
	default:
		return fmt.Errorf("can't walk into kind %v value: %w", info.Value.Kind(), ErrUnknownKind)
	}
}

func (state *walkerState) walkSimple(info *WalkInfo) error {
	return state.callback(info)
}

func (state *walkerState) walkArray(info *WalkInfo) error {
	if err := state.callback(info); err != nil {
		if errors.Is(err, ErrSkip) {
			return nil
		}
		return err
	}

	vLen := info.Value.Len()
	for i := 0; i < vLen; i++ {
		item := info.Value.Index(i)
		itemInfo := newWalkerInfo(item, info)
		if err := state.walkValue(itemInfo); err != nil {
			return err
		}
	}
	return nil
}

func (state *walkerState) walkPtr(info *WalkInfo) error {
	if err := state.callback(info); err != nil {
		if errors.Is(err, ErrSkip) {
			return nil
		}
		return err
	}
	if info.Value.IsNil() {
		return nil
	}
	elem := info.Value.Elem()
	return state.walkValue(newWalkerInfo(elem, info))
}

func (state *walkerState) walkMap(info *WalkInfo) error {
	if err := state.callback(info); err != nil {
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
		keyInfo := newWalkerInfo(key, info)
		keyInfo.isMapKey = true

		if err := state.walkValue(keyInfo); err != nil {
			if errors.Is(err, ErrSkip) {
				continue
			}
			return err
		}

		val := iterator.Value()
		valInfo := newWalkerInfo(val, info)
		valInfo.isMapValue = true
		if err := state.walkValue(valInfo); err != nil {
			return err
		}
	}
	return nil
}

func (state *walkerState) walkSlice(info *WalkInfo) error {
	if err := state.callback(info); err != nil {
		if errors.Is(err, ErrSkip) {
			return nil
		}
		return err
	}

	sliceLen := info.Value.Len()
	for i := 0; i < sliceLen; i++ {
		item := info.Value.Index(i)
		if err := state.walkValue(newWalkerInfo(item, info)); err != nil {
			return err
		}
	}

	return nil
}

func (state *walkerState) walkStruct(info *WalkInfo) error {
	if err := state.callback(info); err != nil {
		if errors.Is(err, ErrSkip) {
			return nil
		}
		return err
	}

	numField := info.Value.NumField()
	for i := 0; i < numField; i++ {
		fieldVal := info.Value.Field(i)
		fieldInfo := newWalkerInfo(fieldVal, info)
		if err := state.walkValue(fieldInfo); err != nil {
			return err
		}
	}

	return nil
}
