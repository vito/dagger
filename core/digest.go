package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"

	"github.com/moby/buildkit/solver/pb"
	"github.com/opencontainers/go-digest"
)

func stableDigest(value any) (digest.Digest, error) {
	buf := new(bytes.Buffer)

	if err := digestInto(value, buf); err != nil {
		return "", err
	}

	return digest.FromReader(buf)
}

func digestInto(value any, dest io.Writer) (err error) {
	defer func() {
		if err := recover(); err != nil {
			panic(fmt.Errorf("digest %T: %v", value, err))
		}
	}()

	switch x := value.(type) {
	case *pb.Definition:
		if x == nil {
			break
		}

		// sort Def since it's in unstable topographical order
		cp := *x
		cp.Def = append([][]byte{}, x.Def...)
		sort.Slice(cp.Def, func(i, j int) bool {
			return bytes.Compare(cp.Def[i], cp.Def[j]) < 0
		})
		value = &cp

	case []byte:
		// base64-encode bytes rather than treating it like a slice
		return json.NewEncoder(dest).Encode(value)
	}

	rt := reflect.TypeOf(value)
	rv := reflect.ValueOf(value)
	if rt.Kind() == reflect.Ptr {
		if rv.IsNil() {
			_, err := fmt.Fprintln(dest, "nil")
			return err
		}
		rt = rt.Elem()
		rv = rv.Elem()
	}

	switch rt.Kind() {
	case reflect.Map:
		if err := digestMapInto(rt, rv, dest); err != nil {
			return fmt.Errorf("digest map: %w", err)
		}
	case reflect.Struct:
		if err := digestStructInto(rt, rv, dest); err != nil {
			return fmt.Errorf("digest struct: %w", err)
		}
	case reflect.Slice, reflect.Array:
		if err := digestSliceInto(rt, rv, dest); err != nil {
			return fmt.Errorf("digest slice/array: %w", err)
		}
	case reflect.String,
		reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		if err := json.NewEncoder(dest).Encode(value); err != nil {
			return err
		}
	default:
		return fmt.Errorf("don't know how to digest %T", value)
	}

	return nil
}

func digestStructInto(rt reflect.Type, rv reflect.Value, dest io.Writer) error {
	for i := 0; i < rt.NumField(); i++ {
		name := rt.Field(i).Name
		fmt.Fprintln(dest, name)
		if err := digestInto(rv.Field(i).Interface(), dest); err != nil {
			return fmt.Errorf("field %s: %w", name, err)
		}
	}

	return nil
}

func digestSliceInto(rt reflect.Type, rv reflect.Value, dest io.Writer) error {
	for i := 0; i < rv.Len(); i++ {
		fmt.Fprintln(dest, i)
		if err := digestInto(rv.Index(i).Interface(), dest); err != nil {
			return fmt.Errorf("index %d: %w", i, err)
		}
	}

	return nil
}

func digestMapInto(rt reflect.Type, rv reflect.Value, dest io.Writer) error {
	keys := rv.MapKeys()
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].String() < keys[j].String()
	})

	for _, k := range keys {
		if err := digestInto(k.Interface(), dest); err != nil {
			return fmt.Errorf("key %v: %w", k, err)
		}
		if err := digestInto(rv.MapIndex(k).Interface(), dest); err != nil {
			return fmt.Errorf("value for key %v: %w", k, err)
		}
	}

	return nil
}
