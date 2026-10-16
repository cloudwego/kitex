package client

import (
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/cloudwego/gopkg/protocol/thrift"
)

var nativeFastTypes sync.Map

func checkNativeFastCodec(data interface{}) error {
	if _, ok := data.(thrift.FastCodec); !ok {
		return fmt.Errorf("native transport requires default generated Fast codec: %T", data)
	}
	t := reflect.TypeOf(data)
	if _, ok := nativeFastTypes.Load(t); ok {
		return nil
	}
	if err := checkNativeFastType(t, make(map[reflect.Type]bool)); err != nil {
		return err
	}
	nativeFastTypes.Store(t, struct{}{})
	return nil
}

func checkNativeFastType(t reflect.Type, visited map[reflect.Type]bool) error {
	if visited[t] {
		return nil
	}
	visited[t] = true
	switch t.Kind() {
	case reflect.Ptr, reflect.Slice, reflect.Array:
		return checkNativeFastType(t.Elem(), visited)
	case reflect.Map:
		if err := checkNativeFastType(t.Key(), visited); err != nil {
			return err
		}
		return checkNativeFastType(t.Elem(), visited)
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.PkgPath != "" {
				continue
			}
			tag := strings.Split(f.Tag.Get("thrift"), ",")
			if len(tag) < 2 || strings.Contains(","+f.Tag.Get("frugal")+",", ",nocopy,") {
				return fmt.Errorf("native transport requires default generated Fast codec: unsupported field %v.%s", t, f.Name)
			}
			if _, ok := reflect.PointerTo(t).MethodByName("FastReadField" + strings.ReplaceAll(tag[1], "-", "_")); !ok {
				return fmt.Errorf("native transport requires default generated Fast codec: missing field decoder for %v.%s", t, f.Name)
			}
			if err := checkNativeFastType(f.Type, visited); err != nil {
				return err
			}
		}
	}
	return nil
}
