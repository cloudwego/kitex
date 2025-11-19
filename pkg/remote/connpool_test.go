/*
 * Copyright 2025 CloudWeGo Authors
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

package remote

import (
	"context"
	"testing"
)

// mockConnStatistics implements ConnStatistics interface for testing
type mockConnStatistics struct {
	activeStreams map[string]int
}

func (m *mockConnStatistics) ActiveStreams(addr string) int {
	if m.activeStreams == nil {
		return 0
	}
	return m.activeStreams[addr]
}

func TestNewCtxWithConnStatistics(t *testing.T) {
	// Test with nil ConnStatistics - should return original context
	ctx := context.Background()
	newCtx := NewCtxWithConnStatistics(ctx, nil)
	if newCtx != ctx {
		t.Errorf("NewCtxWithConnStatistics with nil should return original context")
	}

	// Test with valid ConnStatistics
	mockStats := &mockConnStatistics{
		activeStreams: map[string]int{
			"addr1": 10,
			"addr2": 20,
		},
	}
	newCtx = NewCtxWithConnStatistics(ctx, mockStats)
	if newCtx == ctx {
		t.Errorf("NewCtxWithConnStatistics with valid stats should return new context")
	}

	// Verify the stats can be retrieved
	retrieved := GetConnStatistics(newCtx)
	if retrieved == nil {
		t.Fatalf("GetConnStatistics should return non-nil for context with stats")
	}
	if retrieved.ActiveStreams("addr1") != 10 {
		t.Errorf("Expected 10 active streams for addr1, got %d", retrieved.ActiveStreams("addr1"))
	}
	if retrieved.ActiveStreams("addr2") != 20 {
		t.Errorf("Expected 20 active streams for addr2, got %d", retrieved.ActiveStreams("addr2"))
	}
}

func TestGetConnStatistics(t *testing.T) {
	// Test with empty context - should return nil
	ctx := context.Background()
	stats := GetConnStatistics(ctx)
	if stats != nil {
		t.Errorf("GetConnStatistics on empty context should return nil")
	}

	// Test with context containing ConnStatistics
	mockStats := &mockConnStatistics{
		activeStreams: map[string]int{
			"addr1": 5,
		},
	}
	ctx = NewCtxWithConnStatistics(ctx, mockStats)
	stats = GetConnStatistics(ctx)
	if stats == nil {
		t.Fatalf("GetConnStatistics should return non-nil")
	}
	if stats.ActiveStreams("addr1") != 5 {
		t.Errorf("Expected 5 active streams, got %d", stats.ActiveStreams("addr1"))
	}

	// Test with unknown address
	if stats.ActiveStreams("unknown") != 0 {
		t.Errorf("Expected 0 active streams for unknown address, got %d", stats.ActiveStreams("unknown"))
	}
}

func TestGetConnStatistics_WithWrongType(t *testing.T) {
	// Test with context containing wrong type value
	ctx := context.WithValue(context.Background(), connStatisticsKey, "not a ConnStatistics")
	stats := GetConnStatistics(ctx)
	if stats != nil {
		t.Errorf("GetConnStatistics with wrong type should return nil")
	}
}

func TestConnStatistics_MultipleContextChain(t *testing.T) {
	// Test with context chain
	baseCtx := context.Background()

	mockStats1 := &mockConnStatistics{
		activeStreams: map[string]int{
			"addr1": 10,
		},
	}
	ctx1 := NewCtxWithConnStatistics(baseCtx, mockStats1)

	// Override with new stats
	mockStats2 := &mockConnStatistics{
		activeStreams: map[string]int{
			"addr1": 20,
		},
	}
	ctx2 := NewCtxWithConnStatistics(ctx1, mockStats2)

	// Should get the most recent stats
	stats := GetConnStatistics(ctx2)
	if stats == nil {
		t.Fatalf("GetConnStatistics should return non-nil")
	}
	if stats.ActiveStreams("addr1") != 20 {
		t.Errorf("Expected 20 active streams (from ctx2), got %d", stats.ActiveStreams("addr1"))
	}

	// Original context should still have original stats
	stats1 := GetConnStatistics(ctx1)
	if stats1 == nil {
		t.Fatalf("GetConnStatistics on ctx1 should return non-nil")
	}
	if stats1.ActiveStreams("addr1") != 10 {
		t.Errorf("Expected 10 active streams (from ctx1), got %d", stats1.ActiveStreams("addr1"))
	}
}

func BenchmarkNewCtxWithConnStatistics(b *testing.B) {
	ctx := context.Background()
	mockStats := &mockConnStatistics{
		activeStreams: map[string]int{
			"addr1": 10,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewCtxWithConnStatistics(ctx, mockStats)
	}
}

func BenchmarkGetConnStatistics(b *testing.B) {
	ctx := context.Background()
	mockStats := &mockConnStatistics{
		activeStreams: map[string]int{
			"addr1": 10,
		},
	}
	ctx = NewCtxWithConnStatistics(ctx, mockStats)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GetConnStatistics(ctx)
	}
}
