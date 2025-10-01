package ssestream

import (
	"encoding/json"
	"io"
	"reflect"
	"runtime"
)

func getPointer(v any) any {
	if v == nil {
		return nil
	}
	vv := reflect.ValueOf(v)
	if vv.Kind() == reflect.Ptr {
		return v
	}
	return reflect.New(vv.Type()).Interface()
}

func inferType(v interface{}) reflect.Type {
	return reflect.Indirect(reflect.ValueOf(v)).Type()
}

func newInterface(v interface{}) interface{} {
	if v == nil {
		return nil
	}
	return reflect.New(inferType(v)).Interface()
}

func functionName(i any) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}

func closeq(v any) {
	if c, ok := v.(io.Closer); ok {
		silently(c.Close())
	}
}

func silently(_ ...any) {}

func decodeJSON(r io.Reader, v any) error {
	dec := json.NewDecoder(r)
	for {
		if err := dec.Decode(v); err == io.EOF {
			break
		} else if err != nil {
			return err
		}
	}
	return nil
}
