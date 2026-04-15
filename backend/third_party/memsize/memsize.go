package memsize

import (
	"reflect"
)

// Sizes is a stubbed version of github.com/fjl/memsize.Sizes.
// The original library relies on runtime internals that break on Go 1.23+.
// For our project (0G client dependency via go-ethereum), memsize is only used
// for an optional debug HTTP handler; returning zeros is sufficient.
type Sizes struct {
	Total             uintptr
	ByType            map[reflect.Type]*TypeSize
	BitmapSize        uintptr
	BitmapUtilization float32
}

type TypeSize struct {
	Total uintptr
	Count uintptr
}

func Scan(_ interface{}) Sizes {
	return Sizes{ByType: map[reflect.Type]*TypeSize{}}
}

func HumanSize(_ uintptr) string { return "disabled" }

func (s Sizes) Report() string {
	_ = s
	return "memsize disabled (stubbed for Go 1.23 compatibility)"
}
