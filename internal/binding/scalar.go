package binding

import (
	"encoding"
	"fmt"
	"reflect"
	"strconv"
	"time"
)

var textUnmarshalerType = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()
var timeType = reflect.TypeOf(time.Time{})

func setScalar(target reflect.Value, text string) error {
	if target.Kind() == reflect.Pointer {
		if target.IsNil() {
			target.Set(reflect.New(target.Type().Elem()))
		}
		return setScalar(target.Elem(), text)
	}
	if target.CanAddr() && target.Addr().Type().Implements(textUnmarshalerType) {
		return target.Addr().Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(text))
	}
	if target.Type().Implements(textUnmarshalerType) {
		return target.Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(text))
	}
	if target.Type() == timeType {
		parsed, err := time.Parse(time.RFC3339, text)
		if err != nil {
			return err
		}
		target.Set(reflect.ValueOf(parsed))
		return nil
	}
	switch target.Kind() {
	case reflect.String:
		target.SetString(text)
	case reflect.Bool:
		v, e := strconv.ParseBool(text)
		if e != nil {
			return e
		}
		target.SetBool(v)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v, e := strconv.ParseInt(text, 10, target.Type().Bits())
		if e != nil {
			return e
		}
		target.SetInt(v)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v, e := strconv.ParseUint(text, 10, target.Type().Bits())
		if e != nil {
			return e
		}
		target.SetUint(v)
	case reflect.Float32, reflect.Float64:
		v, e := strconv.ParseFloat(text, target.Type().Bits())
		if e != nil {
			return e
		}
		target.SetFloat(v)
	default:
		return fmt.Errorf("unsupported scalar type %s", target.Type())
	}
	return nil
}
func setValues(target reflect.Value, values []string) error {
	if target.Kind() == reflect.Slice {
		result := reflect.MakeSlice(target.Type(), len(values), len(values))
		for i, v := range values {
			if err := setScalar(result.Index(i), v); err != nil {
				return err
			}
		}
		target.Set(result)
		return nil
	}
	if target.Kind() == reflect.Array {
		if len(values) != target.Len() {
			return fmt.Errorf("expected %d values, got %d", target.Len(), len(values))
		}
		for i, v := range values {
			if err := setScalar(target.Index(i), v); err != nil {
				return err
			}
		}
		return nil
	}
	if len(values) > 1 {
		return fmt.Errorf("multiple values are not allowed")
	}
	if len(values) == 0 {
		return nil
	}
	return setScalar(target, values[0])
}

func setConstrainedValues(target reflect.Value, values []string, constraint string) error {
	if target.Kind() == reflect.Slice {
		result := reflect.MakeSlice(target.Type(), len(values), len(values))
		for i, value := range values {
			if err := setConstrainedScalar(result.Index(i), value, constraint); err != nil {
				return err
			}
		}
		target.Set(result)
		return nil
	}
	if target.Kind() == reflect.Array {
		if len(values) != target.Len() {
			return fmt.Errorf("expected %d values, got %d", target.Len(), len(values))
		}
		for i, value := range values {
			if err := setConstrainedScalar(target.Index(i), value, constraint); err != nil {
				return err
			}
		}
		return nil
	}
	if len(values) > 1 {
		return fmt.Errorf("multiple values are not allowed")
	}
	if len(values) == 0 {
		return nil
	}
	return setConstrainedScalar(target, values[0], constraint)
}

func setConstrainedScalar(target reflect.Value, text, constraint string) error {
	if target.Kind() == reflect.Pointer {
		if target.IsNil() {
			target.Set(reflect.New(target.Type().Elem()))
		}
		return setConstrainedScalar(target.Elem(), text, constraint)
	}
	if target.Type() == timeType {
		layout := time.RFC3339
		if constraint == "date" {
			layout = "2006-01-02"
		}
		parsed, err := time.Parse(layout, text)
		if err != nil {
			return err
		}
		target.Set(reflect.ValueOf(parsed))
		return nil
	}
	return setScalar(target, text)
}
