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

package loadbalance

import (
	"context"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/discovery"
	"github.com/cloudwego/kitex/pkg/remote"
)

// mockConnStatistics implements remote.ConnStatistics for testing
type mockConnStatistics struct {
	activeStreams map[string]int
}

func (m *mockConnStatistics) ActiveStreams(addr string) int {
	if m.activeStreams == nil {
		return 0
	}
	return m.activeStreams[addr]
}

func TestWeightedLeastConnBalancer_GetPicker(t *testing.T) {
	balancer := NewWeightedLeastConnBalancer()

	// Test with nil instances
	picker := balancer.GetPicker(discovery.Result{})
	test.Assert(t, picker != nil)
	dp, ok := picker.(*DummyPicker)
	test.Assert(t, ok && dp != nil)

	// Test with invalid weights
	picker = balancer.GetPicker(discovery.Result{
		Instances: []discovery.Instance{
			discovery.NewInstance("tcp", "addr1", -10, nil),
			discovery.NewInstance("tcp", "addr2", -20, nil),
		},
	})
	test.Assert(t, picker != nil)
	dp, ok = picker.(*DummyPicker)
	test.Assert(t, ok && dp != nil)

	// Test with one instance
	insList := []discovery.Instance{
		discovery.NewInstance("tcp", "addr1", 10, nil),
	}
	picker = balancer.GetPicker(discovery.Result{
		Instances: insList,
	})
	test.Assert(t, picker != nil)

	// Test with multiple instances
	insList = []discovery.Instance{
		discovery.NewInstance("tcp", "addr1", 10, nil),
		discovery.NewInstance("tcp", "addr2", 20, nil),
		discovery.NewInstance("tcp", "addr3", 30, nil),
	}
	picker = balancer.GetPicker(discovery.Result{
		Instances: insList,
	})
	test.Assert(t, picker != nil)

	// Test with balanced instances
	insList = []discovery.Instance{
		discovery.NewInstance("tcp", "addr1", 10, nil),
		discovery.NewInstance("tcp", "addr2", 10, nil),
		discovery.NewInstance("tcp", "addr3", 10, nil),
	}
	picker = balancer.GetPicker(discovery.Result{
		Instances: insList,
	})
	test.Assert(t, picker != nil)
	test.Assert(t, balancer.Name() == "weight_least_conn")
}

func TestWeightedLeastConnPicker_PingPongMode(t *testing.T) {
	balancer := NewWeightedLeastConnBalancer()

	// Test Ping-Pong mode (no ConnStatistics in context)
	ctx := context.Background()

	// Test with empty instances
	picker := balancer.GetPicker(discovery.Result{
		Instances: make([]discovery.Instance, 0),
	})
	ins := picker.Next(ctx, nil)
	test.Assert(t, ins == nil)

	// Test with one instance
	insList := []discovery.Instance{
		discovery.NewInstance("tcp", "addr1", 10, nil),
	}
	picker = balancer.GetPicker(discovery.Result{
		Instances: insList,
	})
	ins = picker.Next(ctx, nil)
	test.Assert(t, ins != nil)
	test.Assert(t, ins.Address().String() == "addr1")

	// Test with multiple instances (should use weighted round-robin for unbalanced)
	insList = []discovery.Instance{
		discovery.NewInstance("tcp", "addr1", 10, nil),
		discovery.NewInstance("tcp", "addr2", 20, nil),
		discovery.NewInstance("tcp", "addr3", 30, nil),
	}
	picker = balancer.GetPicker(discovery.Result{
		Instances: insList,
	})

	// Verify picker uses weighted round-robin logic for unbalanced instances
	pickedAddrs := make(map[string]int)
	for i := 0; i < 60; i++ { // 60 = LCM of weights (10,20,30)
		ins := picker.Next(ctx, nil)
		test.Assert(t, ins != nil)
		pickedAddrs[ins.Address().String()]++
	}

	// Should follow weight ratio 10:20:30 = 1:2:3
	test.Assert(t, pickedAddrs["addr1"] == 10)
	test.Assert(t, pickedAddrs["addr2"] == 20)
	test.Assert(t, pickedAddrs["addr3"] == 30)

	// Test with balanced instances (should use round-robin)
	insList = []discovery.Instance{
		discovery.NewInstance("tcp", "addr1", 10, nil),
		discovery.NewInstance("tcp", "addr2", 10, nil),
		discovery.NewInstance("tcp", "addr3", 10, nil),
	}
	picker = balancer.GetPicker(discovery.Result{
		Instances: insList,
	})

	// Verify picker uses round-robin logic for balanced instances
	pickedAddrs = make(map[string]int)
	for i := 0; i < 30; i++ {
		ins := picker.Next(ctx, nil)
		test.Assert(t, ins != nil)
		pickedAddrs[ins.Address().String()]++
	}

	// Should be evenly distributed
	test.Assert(t, pickedAddrs["addr1"] == 10)
	test.Assert(t, pickedAddrs["addr2"] == 10)
	test.Assert(t, pickedAddrs["addr3"] == 10)
}

func TestWeightedLeastConnPicker_StreamingMode(t *testing.T) {
	balancer := NewWeightedLeastConnBalancer()

	// Create instances with different weights
	insList := []discovery.Instance{
		discovery.NewInstance("tcp", "addr1", 10, nil), // weight 10
		discovery.NewInstance("tcp", "addr2", 20, nil), // weight 20
		discovery.NewInstance("tcp", "addr3", 30, nil), // weight 30
	}

	picker := balancer.GetPicker(discovery.Result{
		Instances: insList,
	})

	// Test Streaming mode (with ConnStatistics in context)
	// Scenario 1: All instances have 0 active streams - should pick based on highest weight/lowest normalized load
	mockStats := &mockConnStatistics{
		activeStreams: map[string]int{
			"addr1": 0, // normalized load: 0/10 = 0
			"addr2": 0, // normalized load: 0/20 = 0
			"addr3": 0, // normalized load: 0/30 = 0
		},
	}
	ctx := remote.NewCtxWithConnStatistics(context.Background(), mockStats)

	// When all have same normalized load (0), should use round robin
	ins := picker.Next(ctx, nil)
	test.Assert(t, ins != nil)
	// Should be one of the instances (round robin behavior when all are 0)
	addr := ins.Address().String()
	test.Assert(t, addr == "addr1" || addr == "addr2" || addr == "addr3")

	// Scenario 2: Different active streams - should pick lowest normalized load
	mockStats = &mockConnStatistics{
		activeStreams: map[string]int{
			"addr1": 5, // normalized load: 5/10 = 0.5
			"addr2": 5, // normalized load: 5/20 = 0.25
			"addr3": 5, // normalized load: 5/30 ≈ 0.167
		},
	}
	ctx = remote.NewCtxWithConnStatistics(context.Background(), mockStats)

	ins = picker.Next(ctx, nil)
	test.Assert(t, ins != nil)
	test.Assert(t, ins.Address().String() == "addr3") // lowest normalized load

	// Scenario 3: One instance heavily loaded
	mockStats = &mockConnStatistics{
		activeStreams: map[string]int{
			"addr1": 10, // normalized load: 10/10 = 1.0
			"addr2": 5,  // normalized load: 5/20 = 0.25
			"addr3": 15, // normalized load: 15/30 = 0.5
		},
	}
	ctx = remote.NewCtxWithConnStatistics(context.Background(), mockStats)

	ins = picker.Next(ctx, nil)
	test.Assert(t, ins != nil)
	test.Assert(t, ins.Address().String() == "addr2") // lowest normalized load

	// Scenario 4: Test multiple picks to ensure consistency
	for i := 0; i < 10; i++ {
		ins = picker.Next(ctx, nil)
		test.Assert(t, ins != nil)
		test.Assert(t, ins.Address().String() == "addr2")
	}
}

func TestWeightedLeastConnPicker_EdgeCases(t *testing.T) {
	balancer := NewWeightedLeastConnBalancer()

	// Test with single instance
	insList := []discovery.Instance{
		discovery.NewInstance("tcp", "addr1", 5, nil),
	}
	picker := balancer.GetPicker(discovery.Result{
		Instances: insList,
	})

	mockStats := &mockConnStatistics{
		activeStreams: map[string]int{
			"addr1": 10, // high load but only option
		},
	}
	ctx := remote.NewCtxWithConnStatistics(context.Background(), mockStats)

	ins := picker.Next(ctx, nil)
	test.Assert(t, ins != nil)
	test.Assert(t, ins.Address().String() == "addr1")

	// Test fallback to ping-pong when ConnStatistics is nil
	ctx = context.Background() // no ConnStatistics
	ins = picker.Next(ctx, nil)
	test.Assert(t, ins != nil)
	test.Assert(t, ins.Address().String() == "addr1")
}

func BenchmarkWeightedLeastConnPicker_PingPong(b *testing.B) {
	balancer := NewWeightedLeastConnBalancer()
	insList := []discovery.Instance{
		discovery.NewInstance("tcp", "addr1", 10, nil),
		discovery.NewInstance("tcp", "addr2", 20, nil),
		discovery.NewInstance("tcp", "addr3", 30, nil),
	}

	picker := balancer.GetPicker(discovery.Result{
		Instances: insList,
	})
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		picker.Next(ctx, nil)
	}
}

func BenchmarkWeightedLeastConnPicker_Streaming(b *testing.B) {
	balancer := NewWeightedLeastConnBalancer()
	insList := []discovery.Instance{
		discovery.NewInstance("tcp", "addr1", 10, nil),
		discovery.NewInstance("tcp", "addr2", 20, nil),
		discovery.NewInstance("tcp", "addr3", 30, nil),
	}

	picker := balancer.GetPicker(discovery.Result{
		Instances: insList,
	})

	mockStats := &mockConnStatistics{
		activeStreams: map[string]int{
			"addr1": 5,
			"addr2": 10,
			"addr3": 2,
		},
	}
	ctx := remote.NewCtxWithConnStatistics(context.Background(), mockStats)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		picker.Next(ctx, nil)
	}
}

// TestWeightedLeastConnPicker_ZeroWeight tests behavior when instance has zero weight
func TestWeightedLeastConnPicker_ZeroWeight(t *testing.T) {
	balancer := NewWeightedLeastConnBalancer()

	// Test with zero weight - should be treated as weight 1
	insList := []discovery.Instance{
		discovery.NewInstance("tcp", "addr1", 0, nil), // zero weight
		discovery.NewInstance("tcp", "addr2", 20, nil),
		discovery.NewInstance("tcp", "addr3", 30, nil),
	}
	picker := balancer.GetPicker(discovery.Result{
		Instances: insList,
	})

	mockStats := &mockConnStatistics{
		activeStreams: map[string]int{
			"addr1": 10, // normalized load: 10/1 = 10 (weight treated as 1)
			"addr2": 10, // normalized load: 10/20 = 0.5
			"addr3": 10, // normalized load: 10/30 ≈ 0.33
		},
	}
	ctx := remote.NewCtxWithConnStatistics(context.Background(), mockStats)

	ins := picker.Next(ctx, nil)
	test.Assert(t, ins != nil)
	// Should pick addr3 with lowest normalized load
	test.Assert(t, ins.Address().String() == "addr3")
}

// TestWeightedLeastConnPicker_NegativeWeight tests behavior when instance has negative weight
func TestWeightedLeastConnPicker_NegativeWeight(t *testing.T) {
	balancer := NewWeightedLeastConnBalancer()

	// Test with negative weight - should be treated as weight 1
	insList := []discovery.Instance{
		discovery.NewInstance("tcp", "addr1", -10, nil), // negative weight
		discovery.NewInstance("tcp", "addr2", 20, nil),
		discovery.NewInstance("tcp", "addr3", 30, nil),
	}
	picker := balancer.GetPicker(discovery.Result{
		Instances: insList,
	})

	mockStats := &mockConnStatistics{
		activeStreams: map[string]int{
			"addr1": 5,  // normalized load: 5/1 = 5 (weight treated as 1)
			"addr2": 10, // normalized load: 10/20 = 0.5
			"addr3": 15, // normalized load: 15/30 = 0.5
		},
	}
	ctx := remote.NewCtxWithConnStatistics(context.Background(), mockStats)

	ins := picker.Next(ctx, nil)
	test.Assert(t, ins != nil)
	// Should pick addr2 or addr3 with lowest normalized load (0.5)
	addr := ins.Address().String()
	test.Assert(t, addr == "addr2" || addr == "addr3")
}

// TestWeightedLeastConnPicker_AllSameLoad tests when all instances have same normalized load
func TestWeightedLeastConnPicker_AllSameLoad(t *testing.T) {
	balancer := NewWeightedLeastConnBalancer()

	insList := []discovery.Instance{
		discovery.NewInstance("tcp", "addr1", 10, nil),
		discovery.NewInstance("tcp", "addr2", 20, nil),
		discovery.NewInstance("tcp", "addr3", 30, nil),
	}
	picker := balancer.GetPicker(discovery.Result{
		Instances: insList,
	})

	// All instances have same normalized load: 1.0
	mockStats := &mockConnStatistics{
		activeStreams: map[string]int{
			"addr1": 10, // normalized load: 10/10 = 1.0
			"addr2": 20, // normalized load: 20/20 = 1.0
			"addr3": 30, // normalized load: 30/30 = 1.0
		},
	}
	ctx := remote.NewCtxWithConnStatistics(context.Background(), mockStats)

	// When all have same load, should use round-robin among all instances
	pickedAddrs := make(map[string]int)
	for i := 0; i < 30; i++ {
		ins := picker.Next(ctx, nil)
		test.Assert(t, ins != nil)
		pickedAddrs[ins.Address().String()]++
	}

	// All instances should be picked (round-robin among candidates with same load)
	for _, addr := range []string{"addr1", "addr2", "addr3"} {
		test.Assert(t, pickedAddrs[addr] > 0, "addr %s should be picked", addr)
	}
	test.Assert(t, pickedAddrs["addr1"]+pickedAddrs["addr2"]+pickedAddrs["addr3"] == 30)
}

// TestWeightedLeastConnPicker_VeryLargeStreamCount tests with very large active stream counts
func TestWeightedLeastConnPicker_VeryLargeStreamCount(t *testing.T) {
	balancer := NewWeightedLeastConnBalancer()

	insList := []discovery.Instance{
		discovery.NewInstance("tcp", "addr1", 100, nil),
		discovery.NewInstance("tcp", "addr2", 200, nil),
		discovery.NewInstance("tcp", "addr3", 300, nil),
	}
	picker := balancer.GetPicker(discovery.Result{
		Instances: insList,
	})

	// Very large stream counts
	mockStats := &mockConnStatistics{
		activeStreams: map[string]int{
			"addr1": 1000000, // normalized load: 1000000/100 = 10000
			"addr2": 1000000, // normalized load: 1000000/200 = 5000
			"addr3": 1000000, // normalized load: 1000000/300 ≈ 3333.33
		},
	}
	ctx := remote.NewCtxWithConnStatistics(context.Background(), mockStats)

	ins := picker.Next(ctx, nil)
	test.Assert(t, ins != nil)
	// Should pick addr3 with lowest normalized load
	test.Assert(t, ins.Address().String() == "addr3")
}

// TestWeightedLeastConnPicker_UnknownAddress tests with address not in stats
func TestWeightedLeastConnPicker_UnknownAddress(t *testing.T) {
	balancer := NewWeightedLeastConnBalancer()

	insList := []discovery.Instance{
		discovery.NewInstance("tcp", "addr1", 10, nil),
		discovery.NewInstance("tcp", "addr2", 20, nil),
		discovery.NewInstance("tcp", "addr3", 30, nil),
	}
	picker := balancer.GetPicker(discovery.Result{
		Instances: insList,
	})

	// Stats missing addr3
	mockStats := &mockConnStatistics{
		activeStreams: map[string]int{
			"addr1": 10, // normalized load: 10/10 = 1.0
			"addr2": 20, // normalized load: 20/20 = 1.0
			// "addr3" is missing, will return 0
		},
	}
	ctx := remote.NewCtxWithConnStatistics(context.Background(), mockStats)

	ins := picker.Next(ctx, nil)
	test.Assert(t, ins != nil)
	// Should pick addr3 with 0 active streams (lowest normalized load: 0/30 = 0)
	test.Assert(t, ins.Address().String() == "addr3")
}

// TestWeightedLeastConnPicker_AvoidTrafficSkew tests that when multiple instances
// have the same minimum load, traffic is distributed evenly among them using round-robin
func TestWeightedLeastConnPicker_AvoidTrafficSkew(t *testing.T) {
	balancer := NewWeightedLeastConnBalancer()

	// Scenario: 1 instance with load, 4 instances with zero load
	// Should distribute new traffic evenly among the 4 zero-load instances
	insList := []discovery.Instance{
		discovery.NewInstance("tcp", "addr1", 10, nil), // has load
		discovery.NewInstance("tcp", "addr2", 10, nil), // zero load
		discovery.NewInstance("tcp", "addr3", 10, nil), // zero load
		discovery.NewInstance("tcp", "addr4", 10, nil), // zero load
		discovery.NewInstance("tcp", "addr5", 10, nil), // zero load
	}

	picker := balancer.GetPicker(discovery.Result{
		Instances: insList,
	})

	mockStats := &mockConnStatistics{
		activeStreams: map[string]int{
			"addr1": 5, // normalized load: 5/10 = 0.5
			"addr2": 0, // normalized load: 0/10 = 0
			"addr3": 0, // normalized load: 0/10 = 0
			"addr4": 0, // normalized load: 0/10 = 0
			"addr5": 0, // normalized load: 0/10 = 0
		},
	}
	ctx := remote.NewCtxWithConnStatistics(context.Background(), mockStats)

	// Pick 100 times and verify distribution
	pickedAddrs := make(map[string]int)
	for i := 0; i < 100; i++ {
		ins := picker.Next(ctx, nil)
		test.Assert(t, ins != nil)
		pickedAddrs[ins.Address().String()]++
	}

	// addr1 should NEVER be picked (it has higher load)
	test.Assert(t, pickedAddrs["addr1"] == 0, "addr1 should not be picked")

	// addr2-5 should be picked roughly evenly (each around 25 times)
	// Allow some variance due to round-robin starting point
	for _, addr := range []string{"addr2", "addr3", "addr4", "addr5"} {
		count := pickedAddrs[addr]
		test.Assert(t, count > 0, "addr %s should be picked at least once", addr)
		// Each should get approximately 25% of 100 picks (25 ± some tolerance)
		test.Assert(t, count >= 20 && count <= 30,
			"addr %s picked %d times, expected around 25", addr, count)
	}

	// Verify total picks
	totalPicks := pickedAddrs["addr2"] + pickedAddrs["addr3"] + pickedAddrs["addr4"] + pickedAddrs["addr5"]
	test.Assert(t, totalPicks == 100)
}

// TestWeightedLeastConnPicker_PartialSameLoad tests distribution when some instances
// have the same minimum load
func TestWeightedLeastConnPicker_PartialSameLoad(t *testing.T) {
	balancer := NewWeightedLeastConnBalancer()

	insList := []discovery.Instance{
		discovery.NewInstance("tcp", "addr1", 10, nil),
		discovery.NewInstance("tcp", "addr2", 10, nil),
		discovery.NewInstance("tcp", "addr3", 10, nil),
		discovery.NewInstance("tcp", "addr4", 10, nil),
	}

	picker := balancer.GetPicker(discovery.Result{
		Instances: insList,
	})

	// addr1 and addr2 have the same minimum load (0.2)
	// addr3 and addr4 have higher load
	mockStats := &mockConnStatistics{
		activeStreams: map[string]int{
			"addr1": 2, // normalized load: 2/10 = 0.2
			"addr2": 2, // normalized load: 2/10 = 0.2
			"addr3": 5, // normalized load: 5/10 = 0.5
			"addr4": 8, // normalized load: 8/10 = 0.8
		},
	}
	ctx := remote.NewCtxWithConnStatistics(context.Background(), mockStats)

	// Pick 60 times
	pickedAddrs := make(map[string]int)
	for i := 0; i < 60; i++ {
		ins := picker.Next(ctx, nil)
		test.Assert(t, ins != nil)
		pickedAddrs[ins.Address().String()]++
	}

	// Only addr1 and addr2 should be picked (they have minimum load)
	test.Assert(t, pickedAddrs["addr3"] == 0, "addr3 should not be picked")
	test.Assert(t, pickedAddrs["addr4"] == 0, "addr4 should not be picked")

	// addr1 and addr2 should be picked roughly evenly
	test.Assert(t, pickedAddrs["addr1"] > 0, "addr1 should be picked")
	test.Assert(t, pickedAddrs["addr2"] > 0, "addr2 should be picked")
	test.Assert(t, pickedAddrs["addr1"]+pickedAddrs["addr2"] == 60)

	// Each should get approximately 50% (30 ± some tolerance)
	for _, addr := range []string{"addr1", "addr2"} {
		count := pickedAddrs[addr]
		test.Assert(t, count >= 25 && count <= 35,
			"addr %s picked %d times, expected around 30", addr, count)
	}
}

// TestWeightedLeastConnPicker_MixedWeightsAndLoads tests various combinations
func TestWeightedLeastConnPicker_MixedWeightsAndLoads(t *testing.T) {
	balancer := NewWeightedLeastConnBalancer()

	insList := []discovery.Instance{
		discovery.NewInstance("tcp", "addr1", 1, nil),   // low weight
		discovery.NewInstance("tcp", "addr2", 50, nil),  // medium weight
		discovery.NewInstance("tcp", "addr3", 100, nil), // high weight
	}
	picker := balancer.GetPicker(discovery.Result{
		Instances: insList,
	})

	// Different scenarios
	testCases := []struct {
		name              string
		activeStreams     map[string]int
		expectedSelection string
	}{
		{
			name: "addr3 with high weight and low streams wins",
			activeStreams: map[string]int{
				"addr1": 1,  // normalized: 1/1 = 1.0
				"addr2": 25, // normalized: 25/50 = 0.5
				"addr3": 10, // normalized: 10/100 = 0.1
			},
			expectedSelection: "addr3",
		},
		{
			name: "addr1 with low weight but zero streams wins",
			activeStreams: map[string]int{
				"addr1": 0,  // normalized: 0/1 = 0
				"addr2": 25, // normalized: 25/50 = 0.5
				"addr3": 10, // normalized: 10/100 = 0.1
			},
			expectedSelection: "addr1",
		},
		{
			name: "addr2 with balanced load wins",
			activeStreams: map[string]int{
				"addr1": 10, // normalized: 10/1 = 10.0
				"addr2": 5,  // normalized: 5/50 = 0.1
				"addr3": 20, // normalized: 20/100 = 0.2
			},
			expectedSelection: "addr2",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockStats := &mockConnStatistics{
				activeStreams: tc.activeStreams,
			}
			ctx := remote.NewCtxWithConnStatistics(context.Background(), mockStats)

			ins := picker.Next(ctx, nil)
			test.Assert(t, ins != nil)
			test.Assert(t, ins.Address().String() == tc.expectedSelection,
				"Expected %s but got %s", tc.expectedSelection, ins.Address().String())
		})
	}
}
