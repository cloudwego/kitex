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
	"errors"
	"strings"
	"testing"
	"time"
)

// Test helpers - simulate various context types

// noCancelContext simulates a noCancel wrapper
type noCancelContext struct {
	Context context.Context // Exported field name
}

func (c noCancelContext) Deadline() (time.Time, bool)       { return time.Time{}, false }
func (c noCancelContext) Done() <-chan struct{}             { return nil }
func (c noCancelContext) Err() error                        { return nil }
func (c noCancelContext) Value(key interface{}) interface{} { return c.Context.Value(key) }

// customContext with non-standard field name
type customContext struct {
	Underlying context.Context // Exported non-standard field name
}

func (c customContext) Deadline() (time.Time, bool) { return c.Underlying.Deadline() }
func (c customContext) Done() <-chan struct{}       { return c.Underlying.Done() }
func (c customContext) Err() error                  { return c.Underlying.Err() }
func (c customContext) Value(key interface{}) interface{} {
	return c.Underlying.Value(key)
}

// loopContext creates a circular reference (for testing loop detection)
type loopContext struct {
	Context context.Context // Exported field
}

func (c *loopContext) Deadline() (time.Time, bool)       { return time.Time{}, false }
func (c *loopContext) Done() <-chan struct{}             { return nil }
func (c *loopContext) Err() error                        { return nil }
func (c *loopContext) Value(key interface{}) interface{} { return nil }

// TestDumpContext_Nil tests nil context handling
func TestDumpContext_Nil(t *testing.T) {
	output := dumpContext(nil)

	if !strings.Contains(output, "<nil>") {
		t.Errorf("Expected output to contain '<nil>', got: %s", output)
	}
}

// TestDumpContext_Background tests background context
func TestDumpContext_Background(t *testing.T) {
	ctx := context.Background()
	output := dumpContext(ctx)

	// Check basic structure
	if !strings.Contains(output, "Type: context.backgroundCtx") &&
		!strings.Contains(output, "Type: context.emptyCtx") {
		t.Errorf("Expected backgroundCtx or emptyCtx type, got: %s", output)
	}

	if !strings.Contains(output, "Done: nil (will block forever)") {
		t.Errorf("Expected 'Done: nil (will block forever)', got: %s", output)
	}

	if !strings.Contains(output, "Err: <nil>") {
		t.Errorf("Expected 'Err: <nil>', got: %s", output)
	}
}

// TestDumpContext_TODO tests TODO context
func TestDumpContext_TODO(t *testing.T) {
	ctx := context.TODO()
	output := dumpContext(ctx)

	if !strings.Contains(output, "Type: context.") {
		t.Errorf("Expected context type, got: %s", output)
	}

	if !strings.Contains(output, "Done: nil (will block forever)") {
		t.Errorf("Expected 'Done: nil (will block forever)', got: %s", output)
	}
}

// TestDumpContext_WithCancel tests cancelled context
func TestDumpContext_WithCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Test before cancel
	output := dumpContext(ctx)
	if !strings.Contains(output, "Done: open (not triggered yet)") {
		t.Errorf("Expected 'Done: open (not triggered yet)' before cancel, got: %s", output)
	}
	if !strings.Contains(output, "Err: <nil>") {
		t.Errorf("Expected 'Err: <nil>' before cancel, got: %s", output)
	}

	// Cancel and test
	cancel()
	output = dumpContext(ctx)

	if !strings.Contains(output, "Done: closed (triggered)") {
		t.Errorf("Expected 'Done: closed (triggered)' after cancel, got: %s", output)
	}
	if !strings.Contains(output, "Err: context canceled") {
		t.Errorf("Expected 'Err: context canceled' after cancel, got: %s", output)
	}

	// Check parent context is shown
	if !strings.Contains(output, "↓ Parent Context:") {
		t.Errorf("Expected parent context to be shown, got: %s", output)
	}
}

// TestDumpContext_WithDeadline tests deadline context
func TestDumpContext_WithDeadline(t *testing.T) {
	// Test future deadline
	futureTime := time.Now().Add(5 * time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), futureTime)
	defer cancel()

	output := dumpContext(ctx)

	if !strings.Contains(output, "Deadline:") {
		t.Errorf("Expected deadline information, got: %s", output)
	}
	if !strings.Contains(output, "(in ") {
		t.Errorf("Expected 'in X' for future deadline, got: %s", output)
	}

	// Test expired deadline
	pastTime := time.Now().Add(-2 * time.Second)
	ctx2, cancel2 := context.WithDeadline(context.Background(), pastTime)
	defer cancel2()

	output2 := dumpContext(ctx2)

	if !strings.Contains(output2, "Deadline:") {
		t.Errorf("Expected deadline information, got: %s", output2)
	}
	if !strings.Contains(output2, "(expired ") && !strings.Contains(output2, "ago)") {
		t.Errorf("Expected 'expired X ago' for past deadline, got: %s", output2)
	}
	if !strings.Contains(output2, "Done: closed (triggered)") {
		t.Errorf("Expected expired deadline to be triggered, got: %s", output2)
	}
}

// TestDumpContext_WithTimeout tests timeout context
func TestDumpContext_WithTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	output := dumpContext(ctx)

	// Should have deadline
	if !strings.Contains(output, "Deadline:") {
		t.Errorf("Expected deadline information, got: %s", output)
	}

	// Should show parent context
	if !strings.Contains(output, "↓ Parent Context:") {
		t.Errorf("Expected parent context, got: %s", output)
	}
}

// TestDumpContext_WithValue tests value context
func TestDumpContext_WithValue(t *testing.T) {
	ctx := context.WithValue(context.Background(), "request-id", "req-12345")
	output := dumpContext(ctx)

	// Should show the value
	if !strings.Contains(output, "Value: request-id = req-12345") {
		t.Errorf("Expected 'Value: request-id = req-12345', got: %s", output)
	}

	// Should show parent context
	if !strings.Contains(output, "↓ Parent Context:") {
		t.Errorf("Expected parent context, got: %s", output)
	}

	// Check background context is at the bottom
	lines := strings.Split(output, "\n")
	foundValueCtx := false
	foundBackground := false
	for _, line := range lines {
		if strings.Contains(line, "Type: *context.valueCtx") {
			foundValueCtx = true
		}
		if strings.Contains(line, "Type: context.backgroundCtx") ||
			strings.Contains(line, "Type: context.emptyCtx") {
			foundBackground = true
		}
	}

	if !foundValueCtx {
		t.Errorf("Expected to find valueCtx type, got: %s", output)
	}
	if !foundBackground {
		t.Errorf("Expected to find background context, got: %s", output)
	}
}

// TestDumpContext_MultipleValues tests multiple value contexts
func TestDumpContext_MultipleValues(t *testing.T) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, "user-id", "user-123")
	ctx = context.WithValue(ctx, "trace-id", "trace-456")
	ctx = context.WithValue(ctx, "request-id", "req-789")

	output := dumpContext(ctx)

	// All values should be present
	if !strings.Contains(output, "request-id") {
		t.Errorf("Expected 'request-id', got: %s", output)
	}
	if !strings.Contains(output, "trace-id") {
		t.Errorf("Expected 'trace-id', got: %s", output)
	}
	if !strings.Contains(output, "user-id") {
		t.Errorf("Expected 'user-id', got: %s", output)
	}

	// Should have multiple parent contexts
	parentCount := strings.Count(output, "↓ Parent Context:")
	if parentCount < 3 {
		t.Errorf("Expected at least 3 parent contexts, got %d", parentCount)
	}
}

// TestDumpContext_LongValue tests value truncation
func TestDumpContext_LongValue(t *testing.T) {
	longValue := strings.Repeat("x", 200)
	ctx := context.WithValue(context.Background(), "long-key", longValue)
	output := dumpContext(ctx)

	// Should be truncated
	if !strings.Contains(output, "...") {
		t.Errorf("Expected value to be truncated with '...', got: %s", output)
	}

	// Should not contain the full value
	if strings.Contains(output, longValue) {
		t.Errorf("Expected value to be truncated, but found full value")
	}
}

// TestDumpContext_NoCancel tests noCancel-style context
func TestDumpContext_NoCancel(t *testing.T) {
	baseCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel the base

	// Wrap with noCancel
	noCancelCtx := noCancelContext{Context: baseCtx}

	output := dumpContext(noCancelCtx)

	// noCancel should show nil Done
	if !strings.Contains(output, "Done: nil (will block forever)") {
		t.Errorf("Expected noCancel to have 'Done: nil (will block forever)', got: %s", output)
	}

	// Should show parent is cancelled
	if !strings.Contains(output, "↓ Parent Context:") {
		t.Errorf("Expected to show parent context, got: %s", output)
	}

	// Parent should be cancelled
	lines := strings.Split(output, "\n")
	foundCancelled := false
	afterParent := false
	for _, line := range lines {
		if strings.Contains(line, "↓ Parent Context:") {
			afterParent = true
		}
		if afterParent && strings.Contains(line, "Done: closed (triggered)") {
			foundCancelled = true
			break
		}
	}

	if !foundCancelled {
		t.Errorf("Expected parent to be cancelled, got: %s", output)
	}
}

// TestDumpContext_ComplexChain tests a complex context chain
func TestDumpContext_ComplexChain(t *testing.T) {
	// Build: background -> value -> cancel -> timeout -> value -> noCancel
	ctx := context.Background()
	ctx = context.WithValue(ctx, "layer1", "base")
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ctx, cancel2 := context.WithTimeout(ctx, 5*time.Second)
	defer cancel2()
	ctx = context.WithValue(ctx, "layer2", "top")
	ctx = noCancelContext{Context: ctx}

	output := dumpContext(ctx)

	// Check all layers are present
	if !strings.Contains(output, "Type: client.noCancelContext") {
		t.Errorf("Expected noCancelContext type, got: %s", output)
	}

	// Should have multiple parent contexts
	parentCount := strings.Count(output, "↓ Parent Context:")
	if parentCount < 4 {
		t.Errorf("Expected at least 4 parent contexts in chain, got %d", parentCount)
	}

	// Should show values
	if !strings.Contains(output, "layer2") {
		t.Errorf("Expected to find 'layer2' value, got: %s", output)
	}
}

// TestDumpContext_CustomContext tests custom context with non-standard field name
func TestDumpContext_CustomContext(t *testing.T) {
	baseCtx := context.WithValue(context.Background(), "test-key", "test-value")
	customCtx := customContext{Underlying: baseCtx}

	output := dumpContext(customCtx)

	// Should detect the parent context even with non-standard field name
	if !strings.Contains(output, "↓ Parent Context:") {
		t.Errorf("Expected to find parent context with custom field name, got: %s", output)
	}

	// Should show the value from base context
	if !strings.Contains(output, "test-value") {
		t.Errorf("Expected to find 'test-value' in chain, got: %s", output)
	}
}

// TestDumpContext_LoopDetection tests loop detection
func TestDumpContext_LoopDetection(t *testing.T) {
	// Create a simple circular reference
	// Note: In practice, circular references in context chains are rare,
	// but this tests the safety mechanism
	baseCtx := context.Background()
	loopCtx := &loopContext{Context: baseCtx}

	// Create a wrapper that points back to create a cycle
	// We'll create: wrapper -> loopCtx -> wrapper (cycle)
	wrapper := &loopContext{Context: loopCtx}
	loopCtx.Context = wrapper

	output := dumpContext(wrapper)

	// Should detect the loop
	if !strings.Contains(output, "(loop detected)") {
		t.Logf("Output:\n%s", output)
		t.Errorf("Expected loop detection message, got: %s", output)
	}
}

// TestDumpContext_ErrorWithCustomError tests custom error types
func TestDumpContext_ErrorWithCustomError(t *testing.T) {
	ctx, cancel := context.WithCancelCause(context.Background())
	customErr := errors.New("custom error: operation failed")
	cancel(customErr)

	output := dumpContext(ctx)

	// Should show the error
	if !strings.Contains(output, "Err:") {
		t.Errorf("Expected error information, got: %s", output)
	}

	// Should be marked as closed
	if !strings.Contains(output, "Done: closed (triggered)") {
		t.Errorf("Expected context to be closed, got: %s", output)
	}
}

// TestDumpContext_Indentation tests proper indentation
func TestDumpContext_Indentation(t *testing.T) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, "key1", "val1")
	ctx = context.WithValue(ctx, "key2", "val2")

	output := dumpContext(ctx)
	lines := strings.Split(output, "\n")

	// Check indentation increases with depth
	foundNested := false
	for i, line := range lines {
		if strings.Contains(line, "↓ Parent Context:") && i+1 < len(lines) {
			nextLine := lines[i+1]
			// Next line should have more indentation
			if len(nextLine)-len(strings.TrimLeft(nextLine, " ")) > 0 {
				foundNested = true
				break
			}
		}
	}

	if !foundNested {
		t.Errorf("Expected proper indentation in nested contexts, got: %s", output)
	}
}

// TestDumpContext_OutputFormat tests overall output format
func TestDumpContext_OutputFormat(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	output := dumpContext(ctx)

	// Required sections
	requiredSections := []string{
		"Type:",
		"Done:",
		"Err:",
	}

	for _, section := range requiredSections {
		if !strings.Contains(output, section) {
			t.Errorf("Expected output to contain '%s', got: %s", section, output)
		}
	}

	// Should be multi-line
	if strings.Count(output, "\n") < 3 {
		t.Errorf("Expected multi-line output, got: %s", output)
	}
}
