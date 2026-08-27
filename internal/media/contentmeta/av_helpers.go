package contentmeta

import "reflect"

func ptr[T any](v T) *T {
	return &v
}

func (r *AVTags) isEmpty() bool {
	if r == nil {
		return true
	}
	v := reflect.ValueOf(*r)
	for i := range v.NumField() {
		f := v.Field(i)
		if f.Kind() == reflect.Bool && f.Bool() {
			return false
		}
		if f.Kind() == reflect.Pointer && !f.IsNil() {
			return false
		}
	}
	return true
}
