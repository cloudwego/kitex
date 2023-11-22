// Copyright 2021 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package thriftgo

import (
	"reflect"
	"testing"
	"text/template"

	"github.com/cloudwego/thriftgo/generator/golang"
	"github.com/cloudwego/thriftgo/plugin"
)

func Test_patcher_patch(t *testing.T) {
	type fields struct {
		noFastAPI   bool
		utils       *golang.CodeUtils
		module      string
		copyIDL     bool
		version     string
		record      bool
		recordCmd   []string
		deepCopyAPI bool
		protocol    string
		fileTpl     *template.Template
	}
	type args struct {
		req *plugin.Request
	}
	tests := []struct {
		name        string
		fields      fields
		args        args
		wantPatches []*plugin.Generated
		wantErr     bool
	}{
		{
			name:   "base",
			fields: fields{module: "github.com/cloudwego/kitex"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &patcher{
				noFastAPI:   tt.fields.noFastAPI,
				utils:       tt.fields.utils,
				module:      tt.fields.module,
				copyIDL:     tt.fields.copyIDL,
				version:     tt.fields.version,
				record:      tt.fields.record,
				recordCmd:   tt.fields.recordCmd,
				deepCopyAPI: tt.fields.deepCopyAPI,
				protocol:    tt.fields.protocol,
				fileTpl:     tt.fields.fileTpl,
			}
			gotPatches, err := p.patch(tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("patcher.patch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotPatches, tt.wantPatches) {
				t.Errorf("patcher.patch() = %v, want %v", gotPatches, tt.wantPatches)
			}
		})
	}
}
