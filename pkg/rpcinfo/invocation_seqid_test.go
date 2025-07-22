/*
 * Copyright 2024 CloudWeGo Authors
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

package rpcinfo

import (
	"context"
	"testing"
)

// TestWithSequenceID tests the WithSequenceID helper function
func TestWithSequenceID(t *testing.T) {
	tests := []struct {
		name     string
		seqID    int32
		expected int32
	}{
		{"positive sequence ID", 12345, 12345},
		{"max int32", 2147483647, 2147483647},
		{"sequence ID 1", 1, 1},
		{"large sequence ID", 999999, 999999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := WithSequenceID(context.Background(), tt.seqID)

			// Verify the sequence ID is stored correctly
			value := ctx.Value(CtxKeySequenceID)
			if value == nil {
				t.Fatal("sequence ID not found in context")
			}

			storedSeqID, ok := value.(int32)
			if !ok {
				t.Fatalf("sequence ID has wrong type: %T", value)
			}

			if storedSeqID != tt.expected {
				t.Errorf("sequence ID mismatch: got %d, want %d", storedSeqID, tt.expected)
			}
		})
	}
}

// TestGetSequenceID tests the GetSequenceID helper function
func TestGetSequenceID(t *testing.T) {
	t.Run("sequence ID exists", func(t *testing.T) {
		expectedSeqID := int32(54321)
		ctx := WithSequenceID(context.Background(), expectedSeqID)

		seqID, ok := GetSequenceID(ctx)
		if !ok {
			t.Fatal("GetSequenceID returned false when sequence ID exists")
		}

		if seqID != expectedSeqID {
			t.Errorf("GetSequenceID returned wrong value: got %d, want %d", seqID, expectedSeqID)
		}
	})

	t.Run("sequence ID does not exist", func(t *testing.T) {
		ctx := context.Background()

		seqID, ok := GetSequenceID(ctx)
		if ok {
			t.Error("GetSequenceID returned true when sequence ID does not exist")
		}

		if seqID != 0 {
			t.Errorf("GetSequenceID returned non-zero value when not set: %d", seqID)
		}
	})

	t.Run("wrong type in context", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), CtxKeySequenceID, "not an int32")

		seqID, ok := GetSequenceID(ctx)
		if ok {
			t.Error("GetSequenceID returned true when value has wrong type")
		}

		if seqID != 0 {
			t.Errorf("GetSequenceID returned non-zero value when type is wrong: %d", seqID)
		}
	})
}

// TestSequenceIDPreservation tests that sequence IDs can be preserved through context
func TestSequenceIDPreservation(t *testing.T) {
	// This test simulates the relay scenario where we want to preserve sequence IDs

	// Step 1: Create an invocation with auto-generated sequence ID
	inv1 := NewInvocation("service1", "method1")
	autoSeqID := inv1.SeqID()
	if autoSeqID == 0 {
		t.Fatal("auto-generated sequence ID should not be 0")
	}
	t.Logf("Auto-generated sequence ID: %d", autoSeqID)

	// Step 2: Store this sequence ID in context (simulating relay extraction)
	ctx := WithSequenceID(context.Background(), autoSeqID)

	// Step 3: In the actual implementation, the client would use this context
	// and the initRPCInfo function would check for the sequence ID
	retrievedSeqID, ok := GetSequenceID(ctx)
	if !ok {
		t.Fatal("Failed to retrieve sequence ID from context")
	}

	if retrievedSeqID != autoSeqID {
		t.Errorf("Retrieved sequence ID mismatch: got %d, want %d", retrievedSeqID, autoSeqID)
	}

	// Step 4: Verify we can create a new invocation and set its sequence ID
	inv2 := NewInvocation("service2", "method2")
	// *invocation implements InvocationSetter, so we can call SetSeqID directly
	inv2.SetSeqID(retrievedSeqID)

	if inv2.SeqID() != autoSeqID {
		t.Errorf("Failed to preserve sequence ID: got %d, want %d", inv2.SeqID(), autoSeqID)
	}
}

// TestContextChaining tests that sequence IDs work with context chaining
func TestContextChaining(t *testing.T) {
	// Start with a base context
	ctx := context.Background()

	// Add sequence ID
	seqID := int32(11111)
	ctx = WithSequenceID(ctx, seqID)

	// Add other values to context (simulating real usage)
	type ctxKey string
	ctx = context.WithValue(ctx, ctxKey("other"), "value")
	ctx = context.WithValue(ctx, ctxKey("another"), 123)

	// Verify sequence ID is still retrievable
	retrievedSeqID, ok := GetSequenceID(ctx)
	if !ok {
		t.Fatal("Failed to retrieve sequence ID after context chaining")
	}

	if retrievedSeqID != seqID {
		t.Errorf("Sequence ID changed after context chaining: got %d, want %d", retrievedSeqID, seqID)
	}
}

// TestZeroSequenceID tests behavior with zero sequence ID
func TestZeroSequenceID(t *testing.T) {
	// Zero sequence ID should be treated as "not set"
	ctx := WithSequenceID(context.Background(), 0)

	// The context will still have the value
	seqID, ok := GetSequenceID(ctx)
	if !ok {
		t.Error("GetSequenceID returned false even though value was set")
	}

	if seqID != 0 {
		t.Errorf("Expected zero sequence ID, got %d", seqID)
	}
}

// TestNegativeSequenceID tests behavior with negative sequence ID
func TestNegativeSequenceID(t *testing.T) {
	// Negative sequence IDs might be used for special purposes
	negSeqID := int32(-1)
	ctx := WithSequenceID(context.Background(), negSeqID)

	seqID, ok := GetSequenceID(ctx)
	if !ok {
		t.Fatal("Failed to store negative sequence ID")
	}

	if seqID != negSeqID {
		t.Errorf("Negative sequence ID not preserved: got %d, want %d", seqID, negSeqID)
	}
}

// BenchmarkWithSequenceID benchmarks the WithSequenceID function
func BenchmarkWithSequenceID(b *testing.B) {
	ctx := context.Background()
	seqID := int32(12345)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = WithSequenceID(ctx, seqID)
	}
}

// BenchmarkGetSequenceID benchmarks the GetSequenceID function
func BenchmarkGetSequenceID(b *testing.B) {
	ctx := WithSequenceID(context.Background(), 12345)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GetSequenceID(ctx)
	}
}
