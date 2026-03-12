/*
 * Copyright 2026 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package client

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"
	"unsafe"
)

func dumpContext(ctx context.Context) string {
	var sb strings.Builder
	visited := make(map[context.Context]struct{})
	doDumpContext(ctx, &sb, 0, visited)
	return sb.String()
}

func doDumpContext(
	ctx context.Context,
	sb *strings.Builder,
	depth int,
	visited map[context.Context]struct{},
) {
	indent := strings.Repeat("  ", depth)

	if ctx == nil {
		fmt.Fprintf(sb, "%s<nil>\n", indent)
		return
	}

	// Loop detection (from v2)
	if _, ok := visited[ctx]; ok {
		fmt.Fprintf(sb, "%s(loop detected) %T\n", indent, ctx)
		return
	}
	visited[ctx] = struct{}{}

	// Type information
	fmt.Fprintf(sb, "%sType: %T\n", indent, ctx)

	// Done state with detailed descriptions (from v1, improved)
	done := ctx.Done()
	var doneStatus string
	if done == nil {
		doneStatus = "nil (will block forever)"
	} else {
		select {
		case <-done:
			doneStatus = "closed (triggered)"
		default:
			doneStatus = "open (not triggered yet)"
		}
	}
	fmt.Fprintf(sb, "%s  Done: %s\n", indent, doneStatus)

	// Error state
	if err := ctx.Err(); err != nil {
		fmt.Fprintf(sb, "%s  Err: %v\n", indent, err)
	} else {
		fmt.Fprintf(sb, "%s  Err: <nil>\n", indent)
	}

	// Deadline information (from v2)
	if dl, ok := ctx.Deadline(); ok {
		remaining := time.Until(dl)
		if remaining > 0 {
			fmt.Fprintf(sb, "%s  Deadline: %v (in %v)\n", indent, dl.Format("15:04:05.000"), remaining.Round(time.Millisecond))
		} else {
			fmt.Fprintf(sb, "%s  Deadline: %v (expired %v ago)\n", indent, dl.Format("15:04:05.000"), (-remaining).Round(time.Millisecond))
		}
	}

	// ValueCtx key/value (from v2, optional)
	doPrintValueCtx(ctx, sb, indent)

	// Extract parent context (using v2's more general approach)
	parent := extractParentContext(ctx)
	if parent != nil && parent != ctx {
		fmt.Fprintf(sb, "%s  ↓ Parent Context:\n", indent) // from v1's style
		doDumpContext(parent, sb, depth+1, visited)
	}
}

func doPrintValueCtx(ctx context.Context, sb *strings.Builder, indent string) {
	v := reflect.ValueOf(ctx)

	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return
	}

	// Check if this is a valueCtx by looking for key and val fields
	keyField := v.FieldByName("key")
	valField := v.FieldByName("val")

	if !keyField.IsValid() || !valField.IsValid() {
		return
	}

	key := readUnexported(keyField)
	val := readUnexported(valField)

	// Format the key nicely
	keyStr := fmt.Sprintf("%v", key)
	if keyStr != "" && keyStr != "<unreadable>" {
		// Truncate very long values
		// Avoid formatting maps directly to prevent concurrent map iteration fatal errors
		valStr := safeFormatValue(val)
		if len(valStr) > 100 {
			valStr = valStr[:97] + "..."
		}
		fmt.Fprintf(sb, "%s  Value: %s = %s\n", indent, keyStr, valStr)
	}
}

func extractParentContext(ctx context.Context) context.Context {
	v := reflect.ValueOf(ctx)

	// If it's a pointer, dereference it
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return nil
	}

	// If the value is not addressable, create an addressable copy
	// This is needed to read unexported fields from value-type contexts
	if !v.CanAddr() {
		// Create a pointer to a copy of the value
		ptr := reflect.New(v.Type())
		ptr.Elem().Set(v)
		v = ptr.Elem()
	}

	// First try common field names (more efficient for known types)
	commonNames := []string{"Context", "ctx", "parent"}
	for _, name := range commonNames {
		field := v.FieldByName(name)
		if !field.IsValid() {
			continue
		}

		fv := readField(field)
		if fv.IsValid() && !fv.IsZero() {
			if parent, ok := fv.Interface().(context.Context); ok {
				return parent
			}
		}
	}

	// Fallback: scan all fields (for custom context types)
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := v.Type().Field(i)

		// Skip if we already checked this field
		isCommonName := false
		for _, name := range commonNames {
			if fieldType.Name == name {
				isCommonName = true
				break
			}
		}
		if isCommonName {
			continue
		}

		fv := readField(field)
		if !fv.IsValid() || fv.IsZero() {
			continue
		}

		if parent, ok := fv.Interface().(context.Context); ok {
			return parent
		}
	}

	return nil
}

func readField(field reflect.Value) reflect.Value {
	if field.CanInterface() {
		return field
	}

	if field.CanAddr() {
		return reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
	}

	return reflect.Value{}
}

func readUnexported(field reflect.Value) interface{} {
	if field.CanInterface() {
		return field.Interface()
	}

	if field.CanAddr() {
		v := reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
		if v.CanInterface() {
			return v.Interface()
		}
	}

	return "<unreadable>"
}

// safeFormatValue safely formats a value, avoiding concurrent map access issues
// by checking the type before formatting
func safeFormatValue(val interface{}) string {
	if val == nil {
		return "<nil>"
	}

	// Use reflection to check the type
	v := reflect.ValueOf(val)
	kind := v.Kind()

	// For map types, avoid direct formatting to prevent concurrent access fatal errors
	// Don't even call Len() as it might not be safe during concurrent modifications
	if kind == reflect.Map {
		return fmt.Sprintf("<map[%v]%v>", v.Type().Key(), v.Type().Elem())
	}

	// For pointer to map, dereference and check
	if kind == reflect.Ptr && !v.IsNil() && v.Elem().Kind() == reflect.Map {
		elem := v.Elem()
		return fmt.Sprintf("<*map[%v]%v>", elem.Type().Key(), elem.Type().Elem())
	}

	// For other types, format normally
	return fmt.Sprintf("%v", val)
}
