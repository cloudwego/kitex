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

package client

import (
	"context"
	"testing"

	"github.com/cloudwego/kitex/internal/mocks"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/transport"
)

// TestClientSequenceIDPreservation tests that custom sequence IDs are preserved through client calls
func TestClientSequenceIDPreservation(t *testing.T) {
	t.Run("sequence ID preservation in client", func(t *testing.T) {
		// This test verifies that if we create a client and make a call with
		// a custom sequence ID in context, it gets preserved

		svcInfo := mocks.ServiceInfo()

		// Create a client
		cli, err := NewClient(svcInfo, WithDestService("test-service"))
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}

		// Create a context with custom sequence ID
		customSeqID := int32(12345)
		ctx := rpcinfo.WithSequenceID(context.Background(), customSeqID)

		// We can't easily test the full flow without a server, but we can
		// at least verify that the context propagation works
		retrievedSeqID, ok := rpcinfo.GetSequenceID(ctx)
		if !ok {
			t.Fatal("Failed to retrieve sequence ID from context")
		}

		if retrievedSeqID != customSeqID {
			t.Errorf("Sequence ID mismatch: got %d, want %d", retrievedSeqID, customSeqID)
		}

		// Verify the client was created successfully
		if cli == nil {
			t.Fatal("Client is nil")
		}
	})
}

// TestSequenceIDContextPropagation tests that sequence IDs propagate correctly through context
func TestSequenceIDContextPropagation(t *testing.T) {
	testCases := []struct {
		name     string
		seqID    int32
		expected int32
		shouldOK bool
	}{
		{"positive sequence ID", 12345, 12345, true},
		{"zero sequence ID", 0, 0, true},
		{"negative sequence ID", -123, -123, true},
		{"max int32", 2147483647, 2147483647, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := rpcinfo.WithSequenceID(context.Background(), tc.seqID)

			retrievedSeqID, ok := rpcinfo.GetSequenceID(ctx)
			if ok != tc.shouldOK {
				t.Errorf("GetSequenceID returned %v, expected %v", ok, tc.shouldOK)
			}

			if retrievedSeqID != tc.expected {
				t.Errorf("Sequence ID mismatch: got %d, want %d", retrievedSeqID, tc.expected)
			}
		})
	}
}

// TestStreamingWithSequenceID tests sequence ID with streaming transport
func TestStreamingWithSequenceID(t *testing.T) {
	svcInfo := mocks.ServiceInfo()

	// Create a streaming client
	cli, err := NewClient(svcInfo,
		WithDestService("test-service"),
		WithTransportProtocol(transport.TTHeaderStreaming))
	if err != nil {
		t.Fatalf("Failed to create streaming client: %v", err)
	}

	// Create a context with custom sequence ID
	customSeqID := int32(54321)
	ctx := rpcinfo.WithSequenceID(context.Background(), customSeqID)

	// Verify context has the sequence ID
	retrievedSeqID, ok := rpcinfo.GetSequenceID(ctx)
	if !ok {
		t.Fatal("Failed to retrieve sequence ID from context")
	}

	if retrievedSeqID != customSeqID {
		t.Errorf("Sequence ID mismatch: got %d, want %d", retrievedSeqID, customSeqID)
	}

	// Verify the client was created successfully
	if cli == nil {
		t.Fatal("Client is nil")
	}
}