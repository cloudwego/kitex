package thrift

import "encoding/json"

func asInt64(v interface{}) (int64, bool) {
	switch v := v.(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	}
	return asInt64Slow(v)
}

func asInt64Slow(v interface{}) (int64, bool) {
	switch v := v.(type) {
	case int8:
		return int64(v), true
	case uint8:
		return int64(v), true
	case int32:
		return int64(v), true
	case uint32:
		return int64(v), true
	case json.Number:
		r, e := v.Int64()
		return r, e == nil
	case int16:
		return int64(v), true
	case uint16:
		return int64(v), true
	case uint:
		return int64(v), (v >> 63) > 0
	case uint64:
		return int64(v), (v >> 63) > 0
	}
	return 0, false
}

func asFloat64(v interface{}) (float64, bool) {
	switch v := v.(type) {
	case float64:
		return v, true
	case json.Number:
		r, e := v.Float64()
		return r, e == nil
	}
	return asFloat64Slow(v)
}

func asFloat64Slow(v interface{}) (float64, bool) {
	if b, ok := v.(float32); ok {
		return float64(b), true
	}
	i64, ok := asInt64(v)
	return float64(i64), ok
}
