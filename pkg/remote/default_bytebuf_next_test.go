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

package remote

import "testing"

func TestDefaultByteBufferNextFailureDoesNotAdvance(t *testing.T) {
	b := NewReaderBuffer([]byte("abc"))
	defer b.Release(nil)
	got, err := b.Next(1)
	if err != nil || string(got) != "a" {
		t.Fatalf("first Next = %q, %v; want a, nil", got, err)
	}
	if _, err := b.Next(3); err == nil {
		t.Fatal("Next(3) should fail when only two bytes remain")
	}
	if got := b.ReadLen(); got != 1 {
		t.Fatalf("ReadLen after failed Next = %d; want 1", got)
	}
	if got := b.ReadableLen(); got != 2 {
		t.Fatalf("ReadableLen after failed Next = %d; want 2", got)
	}
	got, err = b.Next(2)
	if err != nil || string(got) != "bc" {
		t.Fatalf("Next after failure = %q, %v; want bc, nil", got, err)
	}
	if _, err = b.Next(1); err == nil {
		t.Fatal("Next after EOF should fail")
	}
	if got := b.ReadLen(); got != 3 {
		t.Fatalf("ReadLen after EOF = %d; want 3", got)
	}
}
