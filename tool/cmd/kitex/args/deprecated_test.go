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

package args

import (
	"flag"
	"io"
	"strings"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
)

func TestDeprecatedFlag(t *testing.T) {
	f := flag.NewFlagSet("TestDeprecatedFlag", flag.ContinueOnError)

	// ignore print
	f.Usage = func() {}
	f.SetOutput(io.Discard)

	p := &deprecatedFlag{}
	test.Assert(t, p.String() == "")

	f.Var(p, "test", "")
	err := f.Parse([]string{"-test", "hello"})
	test.Assert(t, err != nil)
	test.Assert(t, strings.Contains(err.Error(), errFlagDeprecated.Error()), err)
}
