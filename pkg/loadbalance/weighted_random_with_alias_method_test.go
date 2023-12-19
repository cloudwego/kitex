/*
 * Copyright 2023 CloudWeGo Authors
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
	"fmt"
	"testing"
)

func BenchmarkAliasMethodPickerInit(b *testing.B) {
	n := 10
	for i := 0; i < 4; i++ {
		b.Run(fmt.Sprintf("%dins", n), func(b *testing.B) {
			instances := makeNInstances(n, 1000)
			weightSum := n * 1000

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				picker := &AliasMethodPicker{
					instances: instances,
					weightSum: weightSum,
				}
				picker.init()
			}
		})
		n *= 10
	}
}
