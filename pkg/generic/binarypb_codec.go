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

package generic

import (
	"github.com/cloudwego/kitex/pkg/generic/proto"
)

type binaryPbCodec struct {
	svcName      string
	extra        map[string]string
	readerWriter *proto.RawReaderWriter
}

func newBinaryPbCodec(svcName, packageName string) *binaryPbCodec {
	bpc := &binaryPbCodec{
		svcName:      svcName,
		extra:        make(map[string]string),
		readerWriter: proto.NewRawReaderWriter(),
	}
	bpc.setPackageName(packageName)
	return bpc
}

func (c *binaryPbCodec) getMessageReaderWriter() interface{} {
	return c.readerWriter
}

func (c *binaryPbCodec) Name() string {
	return "BinaryPb"
}

func (c *binaryPbCodec) setPackageName(pkg string) {
	c.extra[packageNameKey] = pkg
}
