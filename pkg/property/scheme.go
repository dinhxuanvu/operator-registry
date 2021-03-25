package property

import (
	"fmt"
	"reflect"
)

func init() {
	skips := Skips("")
	skipRange := SkipRange("")

	scheme = map[reflect.Type]string{
		reflect.TypeOf(&Package{}):         TypePackage,
		reflect.TypeOf(&PackageRequired{}): TypePackageRequired,
		reflect.TypeOf(&Channel{}):         TypeChannel,
		reflect.TypeOf(&GVK{}):             TypeGVK,
		reflect.TypeOf(&GVKRequired{}):     TypeGVKRequired,
		reflect.TypeOf(&skips):             TypeSkips,
		reflect.TypeOf(&skipRange):         TypeSkipRange,
	}
}

var scheme map[reflect.Type]string

func AddToScheme(typ string, p interface{}) {
	t := reflect.TypeOf(p)
	if t.Kind() != reflect.Ptr {
		panic("All types must be pointers to structs.")
	}
	if _, ok := scheme[t]; ok {
		panic(fmt.Sprintf("Scheme already contains registration for %s", t))
	}
	scheme[t] = typ
}
