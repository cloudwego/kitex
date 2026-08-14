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

package status

import (
	"context"
	"errors"
	"fmt"
	"testing"

	spb "google.golang.org/genproto/googleapis/rpc/status"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/codes"
)

func TestSourceString(t *testing.T) {
	test.Assert(t, SourceLocal.String() == "local")
	test.Assert(t, SourceInbound.String() == "inbound")
	test.Assert(t, SourceOutbound.String() == "outbound")
	test.Assert(t, Source(255).String() == "local")
}

func TestSourceDefaults(t *testing.T) {
	// Public constructors produce a status in the current process. A receiving
	// transport overwrites the source when it observes an explicit peer status.
	test.Assert(t, New(codes.Canceled, "x").Source() == SourceLocal)
	test.Assert(t, Newf(codes.Canceled, "x%d", 1).Source() == SourceLocal)
	test.Assert(t, FromProto(&spb.Status{Code: 1}).Source() == SourceLocal)
	test.Assert(t, NewWithSource(codes.Canceled, "x", SourceOutbound).Source() == SourceOutbound)
	test.Assert(t, FromProtoWithSource(&spb.Status{Code: 1}, SourceInbound).Source() == SourceInbound)
	test.Assert(t, Convert(errors.New("x")).Source() == SourceLocal)
	test.Assert(t, FromContextError(context.Canceled).Source() == SourceLocal)

	var nilStatus *Status
	test.Assert(t, nilStatus.Source() == SourceLocal)
	test.Assert(t, nilStatus.Copy() == nil)
	test.Assert(t, nilStatus.WithSource(SourceLocal) == nil)
	test.Assert(t, New(codes.Canceled, "x").WithSource(Source(255)).Source() == SourceLocal)
	test.Assert(t, NewWithSource(codes.Canceled, "x", Source(255)).Source() == SourceLocal)
	test.Assert(t, FromProtoWithSource(&spb.Status{Code: 1}, Source(255)).Source() == SourceLocal)
}

func TestSourceCopySemantics(t *testing.T) {
	shared := New(codes.Canceled, "shared")
	tagged := shared.WithSource(SourceInbound)

	// WithSource is immutable and does not pollute a potentially shared status.
	test.Assert(t, shared.Source() == SourceLocal)
	test.Assert(t, tagged.Source() == SourceInbound)
	test.Assert(t, tagged.Code() == codes.Canceled)
	test.Assert(t, tagged.Message() == "shared")

	// deep copy: mutating the tagged copy must not pollute the shared one
	tagged.AppendMessage("extra")
	test.Assert(t, shared.Message() == "shared", shared.Message())
	test.Assert(t, tagged.Message() == "shared extra", tagged.Message())

	cp := tagged.Copy()
	test.Assert(t, cp != tagged)
	test.Assert(t, cp.Source() == SourceInbound)
	test.Assert(t, cp.Message() == tagged.Message())
}

func TestSourceRoundTrip(t *testing.T) {
	st := New(codes.Canceled, "msg").WithSource(SourceOutbound)

	// Status -> Err -> FromError -> Status
	err := st.Err()
	got, ok := FromError(err)
	test.Assert(t, ok)
	test.Assert(t, got.Source() == SourceOutbound, got.Source())

	// wrapped errors keep the attribution
	wrapped := fmt.Errorf("wrap: %w", err)
	got, ok = FromError(wrapped)
	test.Assert(t, ok)
	test.Assert(t, got.Source() == SourceOutbound, got.Source())

	// WithDetails keeps the attribution
	stD, dErr := st.WithDetails(&spb.Status{Code: 2})
	test.Assert(t, dErr == nil, dErr)
	test.Assert(t, stD.Source() == SourceOutbound)
	test.Assert(t, len(stD.Details()) == 1)
	stD.AppendMessage("extra")
	test.Assert(t, st.Message() == "msg", st.Message())
	test.Assert(t, stD.Message() == "msg extra", stD.Message())

	// Proto()/FromProto() drop the peer attribution by design: Source never
	// crosses the wire, so the newly constructed status defaults to Local.
	test.Assert(t, FromProto(st.Proto()).Source() == SourceLocal)
}

func TestSourceNotInErrorEquality(t *testing.T) {
	a := New(codes.Canceled, "x").WithSource(SourceInbound).Err()
	b := New(codes.Canceled, "x").Err()
	// Error.Is keeps comparing only the proto status
	test.Assert(t, errors.Is(a, b))
	test.Assert(t, errors.Is(b, a))
}
