package singleflightutil

import "golang.org/x/sync/singleflight"

// Group is a wrapper around singleflight.Group to provide a CheckAndDo functionality.
// It is used to ensure that `fn` is executed only once for a given key.
type Group struct {
	singleflight.Group
}

// CheckAndDo implements a double-checked pattern to ensure that `fn` is executed only once for a given key.
// It performs an initial check with `checkFunc` before calling `singleflight.Do`. If the value is found, return.
// Otherwise, call `Do` and execute `checkFunc` again before executing `fn`.
func (g *Group) CheckAndDo(key string, checkFunc func() (any, bool), fn func() (any, error)) (v any, err error, shared bool) {
	if value, loaded := checkFunc(); loaded {
		return value, nil, true
	}
	return g.Do(key, func() (any, error) {
		if value, loaded := checkFunc(); loaded {
			return value, nil
		}
		return fn()
	})
}
