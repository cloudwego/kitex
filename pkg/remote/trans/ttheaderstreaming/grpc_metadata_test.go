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

package ttheaderstreaming

import (
	"context"
	"reflect"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/metadata"
)

func Test_grpcMetadata_setHeader(t *testing.T) {
	t.Run("set-with-empty-md", func(t *testing.T) {
		md := metadata.MD{"k2": []string{"v2"}}
		g := &grpcMetadata{headersToSend: md}
		g.setHeader(metadata.MD{})
		test.Assert(t, reflect.DeepEqual(g.headersToSend, md), g.headersToSend)
	})

	t.Run("set-to-empty-map", func(t *testing.T) {
		md := metadata.MD{"k1": []string{"v1"}}
		g := &grpcMetadata{}
		g.setHeader(md)
		test.Assert(t, reflect.DeepEqual(g.headersToSend, md), g.headersToSend)
	})

	t.Run("set-to-non-empty-map", func(t *testing.T) {
		md := metadata.MD{"k1": []string{"v1"}}
		g := &grpcMetadata{headersToSend: metadata.MD{"k2": []string{"v2"}}}
		g.setHeader(md)
		expected := metadata.MD{"k1": []string{"v1"}, "k2": []string{"v2"}}
		test.Assert(t, reflect.DeepEqual(g.headersToSend, expected), g.headersToSend)
	})
}

func Test_grpcMetadata_setTrailer(t *testing.T) {
	t.Run("set-with-empty-md", func(t *testing.T) {
		md := metadata.MD{"k2": []string{"v2"}}
		g := &grpcMetadata{trailerToSend: md}
		g.setTrailer(metadata.MD{})
		test.Assert(t, reflect.DeepEqual(g.trailerToSend, md), g.trailerToSend)
	})

	t.Run("set-to-empty-map", func(t *testing.T) {
		md := metadata.MD{"k1": []string{"v1"}}
		g := &grpcMetadata{}
		g.setTrailer(md)
		test.Assert(t, reflect.DeepEqual(g.trailerToSend, md), g.trailerToSend)
	})

	t.Run("set-to-non-empty-map", func(t *testing.T) {
		md := metadata.MD{"k1": []string{"v1"}}
		g := &grpcMetadata{trailerToSend: metadata.MD{"k2": []string{"v2"}}}
		g.setTrailer(md)
		expected := metadata.MD{"k1": []string{"v1"}, "k2": []string{"v2"}}
		test.Assert(t, reflect.DeepEqual(g.trailerToSend, expected), g.trailerToSend)
	})
}

func Test_grpcMetadata_getMetadataAsJSON(t *testing.T) {
	t.Run("trailer", func(t *testing.T) {
		g := &grpcMetadata{trailerToSend: metadata.MD{"k1": []string{"v1"}}}
		md, err := g.getMetadataAsJSON(g.trailerToSend)
		test.Assert(t, err == nil, err)
		test.Assert(t, string(md) == `{"k1":["v1"]}`)
	})

	t.Run("empty-header", func(t *testing.T) {
		g := &grpcMetadata{}
		md, err := g.getMetadataAsJSON(nil)
		test.Assert(t, err == nil, err)
		test.Assert(t, md == nil)
	})
}

func Test_grpcMetadata_injectMetadata(t *testing.T) {
	t.Run("inject-header", func(t *testing.T) {
		msg := remote.NewMessage(nil, nil, nil, remote.Stream, remote.Server)
		g := &grpcMetadata{headersToSend: metadata.MD{"k1": []string{"v1"}}}
		err := g.injectMetadata(g.headersToSend, msg)
		test.Assert(t, err == nil, err)

		mdJSON, exists := msg.TransInfo().TransStrInfo()[codec.StrKeyMetaData]
		test.Assert(t, exists)
		test.Assert(t, string(mdJSON) == `{"k1":["v1"]}`)
	})
}

func Test_grpcMetadata_parseMetadata(t *testing.T) {
	t.Run("no-metadata", func(t *testing.T) {
		md := metadata.MD{"k1": []string{"v1"}}
		msg := remote.NewMessage(nil, nil, nil, remote.Stream, remote.Server)
		g := &grpcMetadata{}
		err := g.parseMetadata(msg, &md)
		test.Assert(t, err == nil)
		test.Assert(t, md.Len() == 1)
	})
	t.Run("with-metadata", func(t *testing.T) {
		md := metadata.MD{}
		msg := remote.NewMessage(nil, nil, nil, remote.Stream, remote.Server)
		msg.TransInfo().TransStrInfo()[codec.StrKeyMetaData] = `{"k2":["v2"]}`
		g := &grpcMetadata{}
		err := g.parseMetadata(msg, &md)
		test.Assert(t, err == nil)
		test.Assert(t, md["k2"][0] == "v2")
	})
	t.Run("with-invalid-metadata", func(t *testing.T) {
		md := metadata.MD{"k1": []string{"v1"}}
		msg := remote.NewMessage(nil, nil, nil, remote.Stream, remote.Server)
		msg.TransInfo().TransStrInfo()[codec.StrKeyMetaData] = `invalid`
		g := &grpcMetadata{}
		err := g.parseMetadata(msg, &md)
		test.Assert(t, err != nil)
		test.Assert(t, md.Len() == 1)
	})
}

func Test_grpcMetadata_parseHeader(t *testing.T) {
	msg := remote.NewMessage(nil, nil, nil, remote.Stream, remote.Server)
	msg.TransInfo().TransStrInfo()[codec.StrKeyMetaData] = `{"k1":["v1"]}`
	g := &grpcMetadata{}
	err := g.parseHeader(msg)
	test.Assert(t, err == nil)
	test.Assert(t, g.headersReceived["k1"][0] == "v1")
}

func Test_grpcMetadata_parseTrailer(t *testing.T) {
	msg := remote.NewMessage(nil, nil, nil, remote.Stream, remote.Server)
	msg.TransInfo().TransStrInfo()[codec.StrKeyMetaData] = `{"k1":["v1"]}`
	g := &grpcMetadata{}
	err := g.parseTrailer(msg)
	test.Assert(t, err == nil)
	test.Assert(t, g.trailerReceived["k1"][0] == "v1")
}

func Test_grpcMetadata_loadHeadersToSend(t *testing.T) {
	t.Run("no-metadata-in-ctx", func(t *testing.T) {
		g := &grpcMetadata{}
		g.loadHeadersToSend(context.Background())
		test.Assert(t, len(g.headersToSend) == 0)
	})

	t.Run("with-metadata-in-ctx", func(t *testing.T) {
		g := &grpcMetadata{}
		ctx := metadata.AppendToOutgoingContext(context.Background(), "k1", "v1")
		g.loadHeadersToSend(ctx)
		test.Assert(t, len(g.headersToSend) == 1)
		test.Assert(t, g.headersToSend["k1"][0] == "v1")
	})
}
