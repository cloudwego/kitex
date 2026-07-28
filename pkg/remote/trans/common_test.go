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

package trans

import (
	"errors"
	"net"
	"testing"

	"github.com/cloudwego/kitex/internal/mocks"
	"github.com/cloudwego/kitex/internal/test"
)

// connWithActive is a net.Conn that also reports its active state,
// used to simulate a netpoll-like connection in tests.
type connWithActive struct {
	mocks.Conn
	active bool
}

func (c *connWithActive) IsActive() bool {
	return c.active
}

func TestIsRemoteClosedErr(t *testing.T) {
	closedByErr := errors.New("remote closed")
	otherErr := errors.New("some business error")

	// ext recognizes closedByErr as remote-closed, but nothing else.
	ext := &MockExtension{
		IsRemoteClosedErrFunc: func(err error) bool {
			return errors.Is(err, closedByErr)
		},
	}

	activeConn := &connWithActive{active: true}
	inactiveConn := &connWithActive{active: false}
	plainConn := &mocks.Conn{} // does not implement remote.IsActive

	cases := []struct {
		name       string
		err        error
		conn       net.Conn
		wantErr    bool
		wantSource RemoteClosedSource
	}{
		{name: "nil error", err: nil, conn: inactiveConn},
		{name: "ext recognizes the error", err: closedByErr, conn: activeConn, wantErr: true, wantSource: RemoteClosedByExtension},
		{name: "unrecognized error but conn inactive", err: otherErr, conn: inactiveConn, wantErr: true, wantSource: RemoteClosedByConnectionState},
		{name: "unrecognized error and conn active", err: otherErr, conn: activeConn},
		{name: "unrecognized error and conn without IsActive", err: otherErr, conn: plainConn},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := IsRemoteClosedErr(ext, c.err, c.conn)
			if !c.wantErr {
				test.Assert(t, got == nil, c.name, got)
				return
			}
			test.Assert(t, got != nil, c.name)
			test.Assert(t, got.Source == c.wantSource, c.name, got.Source, c.wantSource)
			test.Assert(t, errors.Is(got, ErrRemoteClosed), c.name, got)
			test.Assert(t, errors.Is(got, c.err), c.name, got, c.err)
		})
	}
}
