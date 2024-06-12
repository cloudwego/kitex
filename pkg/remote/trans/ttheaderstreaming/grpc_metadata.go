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
	"encoding/json"
	"fmt"
	"sync"

	"github.com/bytedance/sonic"

	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/metadata"
)

// grpcMetadata contains the grpc style metadata, helpful for projects migrating from kitex-grpc
type grpcMetadata struct {
	lock sync.Mutex

	headersToSend metadata.MD
	trailerToSend metadata.MD

	headersReceived metadata.MD
	trailerReceived metadata.MD
}

func (g *grpcMetadata) setHeader(md metadata.MD) {
	if md.Len() == 0 {
		return
	}
	g.lock.Lock()
	g.headersToSend = metadata.AppendMD(g.headersToSend, md)
	g.lock.Unlock()
}

func (g *grpcMetadata) setTrailer(md metadata.MD) {
	if md.Len() == 0 {
		return
	}
	g.lock.Lock()
	g.trailerToSend = metadata.AppendMD(g.trailerToSend, md)
	g.lock.Unlock()
}

func (g *grpcMetadata) getMetadataAsJSON(md metadata.MD) ([]byte, error) {
	if len(md) == 0 {
		return nil, nil
	}
	return json.Marshal(md)
}

func (g *grpcMetadata) injectHeader(message remote.Message) error {
	return g.injectMetadata(g.headersToSend, message)
}

func (g *grpcMetadata) injectTrailer(message remote.Message) error {
	return g.injectMetadata(g.trailerToSend, message)
}

func (g *grpcMetadata) injectMetadata(md metadata.MD, message remote.Message) error {
	if metadataJSON, err := g.getMetadataAsJSON(md); err != nil {
		return fmt.Errorf("failed to get metadata as json: err=%w", err)
	} else if len(metadataJSON) > 0 {
		message.TransInfo().TransStrInfo()[codec.StrKeyMetaData] = string(metadataJSON)
	}
	return nil
}

func (g *grpcMetadata) parseHeader(message remote.Message) error {
	return g.parseMetadata(message, &g.headersReceived)
}

func (g *grpcMetadata) parseTrailer(message remote.Message) error {
	return g.parseMetadata(message, &g.trailerReceived)
}

func (g *grpcMetadata) parseMetadata(message remote.Message, target *metadata.MD) error {
	if metadataJSON, exists := message.TransInfo().TransStrInfo()[codec.StrKeyMetaData]; exists {
		md := metadata.MD{}
		if err := sonic.Unmarshal([]byte(metadataJSON), &md); err != nil {
			return fmt.Errorf("invalid metadata: json=%s, err = %w", metadataJSON, err)
		}
		g.lock.Lock()
		*target = md
		g.lock.Unlock()
	}
	return nil
}

func (g *grpcMetadata) loadHeadersToSend(ctx context.Context) {
	if md, ok := metadata.FromOutgoingContext(ctx); ok {
		g.headersToSend = md
	}
}
