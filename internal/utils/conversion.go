package utils

import (
	"fmt"
	"github.com/spf13/cast"
	"reflect"
	"time"
)

// Convert Converts the val parameter to the same type as the target
func Convert(val interface{}, target interface{}) (interface{}, error) {
	if val == nil {
		if zero, ok := zeroValueOf(target); ok {
			return zero, nil
		}
		return nil, fmt.Errorf("cannot convert %s to %s", reflect.TypeOf(val), reflect.TypeOf(target))
	}

	switch target.(type) {
	case string:
		return cast.ToStringE(val)
	case int:
		return cast.ToIntE(val)
	case int8:
		return cast.ToInt8E(val)
	case int16:
		return cast.ToInt16E(val)
	case int32:
		return cast.ToInt32E(val)
	case int64:
		return cast.ToInt64E(val)
	case uint:
		return cast.ToUintE(val)
	case uint8:
		return cast.ToUint8E(val)
	case uint16:
		return cast.ToUint16E(val)
	case uint32:
		return cast.ToUint32E(val)
	case uint64:
		return cast.ToUint64E(val)
	case bool:
		return cast.ToBoolE(val)
	case time.Duration:
		return cast.ToDurationE(val)
	case time.Time:
		return cast.ToTimeE(val)
	case float32:
		return cast.ToFloat64E(val)
	case float64:
		return cast.ToFloat64E(val)
	case map[string]string:
		return cast.ToStringMapStringE(val)
	case map[string][]string:
		return cast.ToStringMapStringSliceE(val)
	case map[string]bool:
		return cast.ToStringMapBoolE(val)
	case map[string]interface{}:
		return cast.ToStringMapE(val)
	case map[string]int:
		return cast.ToStringMapIntE(val)
	case map[string]int64:
		return cast.ToStringMapInt64E(val)
	case []interface{}:
		return cast.ToSliceE(val)
	case []bool:
		return cast.ToBoolSliceE(val)
	case []string:
		return cast.ToStringSliceE(val)
	case []int:
		return cast.ToIntSliceE(val)
	case []time.Duration:
		return cast.ToDurationSliceE(val)
	}

	return nil, fmt.Errorf("cannot convert %s to %s", reflect.TypeOf(val), reflect.TypeOf(target))
}

// zeroValueOf returns what a nil value converts to for each supported target type.
func zeroValueOf(target interface{}) (interface{}, bool) {
	switch target.(type) {
	case string:
		return "", true
	case int:
		return int(0), true
	case int8:
		return int8(0), true
	case int16:
		return int16(0), true
	case int32:
		return int32(0), true
	case int64:
		return int64(0), true
	case uint:
		return uint(0), true
	case uint8:
		return uint8(0), true
	case uint16:
		return uint16(0), true
	case uint32:
		return uint32(0), true
	case uint64:
		return uint64(0), true
	case bool:
		return false, true
	case time.Duration:
		return time.Duration(0), true
	case time.Time:
		return time.Time{}, true
	case float32:
		return float32(0), true
	case float64:
		return float64(0), true
	case map[string]string:
		return map[string]string{}, true
	case map[string][]string:
		return map[string][]string{}, true
	case map[string]bool:
		return map[string]bool{}, true
	case map[string]interface{}:
		return map[string]interface{}{}, true
	case map[string]int:
		return map[string]int{}, true
	case map[string]int64:
		return map[string]int64{}, true
	case []interface{}:
		return []interface{}{}, true
	case []bool:
		return []bool{}, true
	case []string:
		return []string{}, true
	case []int:
		return []int{}, true
	case []time.Duration:
		return []time.Duration{}, true
	}
	return nil, false
}
