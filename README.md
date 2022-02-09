[![Go Reference](https://pkg.go.dev/badge/github.com/rekby/objwalker.svg)](https://pkg.go.dev/github.com/rekby/objwalker)
[![Coverage Status](https://coveralls.io/repos/github/rekby/objwalker/badge.svg?branch=master)](https://coveralls.io/github/rekby/objwalker?branch=master)
[![GoReportCard](https://goreportcard.com/badge/github.com/rekby/objwalker)](https://goreportcard.com/report/github.com/rekby/objwalker)

Deep walk by object with reflection. With recursive loop protection.

WalkInfo - struct, send as argument to callback function include:

```Value``` - reflection.Value object for read/manipulate with it.
```InternalStructSize``` - size of internal structs (slice/string header, map struct at first level, ...)
```DataPointer``` - direct pointer to underly data, for example - pointer to bytes under string, ot pointer to data under slice. It is danger to manipulate it, but can userful for example for compare objects.

```golang
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
```
