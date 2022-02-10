[![Go Reference](https://pkg.go.dev/badge/github.com/rekby/objwalker.svg)](https://pkg.go.dev/github.com/rekby/objwalker)
[![Coverage Status](https://coveralls.io/repos/github/rekby/objwalker/badge.svg?branch=master)](https://coveralls.io/github/rekby/objwalker?branch=master)
[![GoReportCard](https://goreportcard.com/badge/github.com/rekby/objwalker)](https://goreportcard.com/report/github.com/rekby/objwalker)

Deep walk by object with reflection. Walker.Walk(v interface{}) call callback function for v object, every field if it struct, every item for slice/array and every key and item for map object. It walk for object recursive and call callback for every object in tree.

It has loop protection - for not hang on cycled structured, protection can be disabled if need.

WalkInfo - struct, send as argument to callback function include:

* ```Value``` - reflection.Value object for read/manipulate with it.
* ```DataPointer``` - direct pointer to underly data, for example - pointer to bytes under string, ot pointer to data under slice. It is danger to manipulate it, but can userful for example for compare objects.
* ```Parent``` - parent of the value in travel tree
* and some other hints about Value

```golang
    package main

    import "github.com/rekby/objwalker"

    func main() {
		type S struct {
			Val1  int
			Slice []string
		}

		val := S{
			Val1:  2,
			Slice: []string{"hello", "world"},
		}
		_ = New(func(info *WalkInfo) error {
			fmt.Println(info.Value.Interface())
			return nil
		}).Walk(val)

		// Output:
		// {2 [hello world]}
		// 2
		// [hello world]
		// hello
		// world
	}
```
