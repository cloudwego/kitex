// Copyright 2025 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package thriftgo

import (
	"sort"
	"strings"

	"github.com/cloudwego/kitex/tool/internal_pkg/util"

	"github.com/cloudwego/thriftgo/parser"
)

const (
	AnnotationQuery   = "api.query"
	AnnotationForm    = "api.form"
	AnnotationPath    = "api.path"
	AnnotationHeader  = "api.header"
	AnnotationCookie  = "api.cookie"
	AnnotationBody    = "api.body"
	AnnotationRawBody = "api.raw_body"
)

var bindingTags = map[string]string{
	AnnotationPath:    "path",
	AnnotationQuery:   "query",
	AnnotationHeader:  "header",
	AnnotationCookie:  "cookie",
	AnnotationBody:    "form",
	AnnotationForm:    "form",
	AnnotationRawBody: "raw_body",
}

type genHTTPTagOption struct {
	// thriftgo: -thrift snake_type_json_tag
	snakeTyleJSONTag bool
	// thriftgo: -thrift low_camel_case_json
	lowerCamelCaseJSONTag bool
	// hertz: --unset_omitempty
	unsetOmitempty bool
	// hertz: --snake_tag
	snakeStyleHTTPTag bool
}

// genHTTPTags append api.xx and json tag into struct field for http binding usage.
func genHTTPTags(f *parser.Field, opt genHTTPTagOption) string {
	w := initTagWriter(f, opt)
	var found bool
	for _, a := range f.Annotations {
		if tag, ok := bindingTags[a.Key]; ok {
			tagVal := a.Values[len(a.Values)-1]
			if a.Key == AnnotationBody {
				w.resetJsonVal(tagVal)
			}
			w.addTag(tag, tagVal)
			found = true
		}
	}
	if !found {
		w.addTag("form", f.Name)
		w.addTag("query", f.Name)
	}
	return w.dump()
}

type tagWriter struct {
	jsonVal string
	tags    map[string]string
	opt     genHTTPTagOption
	f       *parser.Field
}

func (tw *tagWriter) resetJsonVal(jsonVal string) {
	tw.jsonVal = jsonVal
}

func (tw *tagWriter) addTag(k, v string) {
	tw.tags[k] = v
}

func initTagWriter(f *parser.Field, opt genHTTPTagOption) *tagWriter {
	return &tagWriter{f.Name, make(map[string]string), opt, f}
}

func (tw tagWriter) dump() string {
	var tagSuffix, jsonSuffix string
	if tw.f.GetRequiredness().IsRequired() {
		tagSuffix = ",required"
		jsonSuffix = ",required"
	} else if tw.f.GetRequiredness().IsOptional() && !tw.opt.unsetOmitempty {
		jsonSuffix = ",omitempty"
	}
	// same as github.com/cloudwego/thriftgo/generator/golang/utils.go genFieldTags
	if tw.opt.snakeTyleJSONTag {
		tw.jsonVal = util.Snakify(tw.jsonVal)
	}
	if tw.opt.lowerCamelCaseJSONTag {
		tw.jsonVal = util.LowerCamelCase(tw.jsonVal)
	}
	var sb strings.Builder
	tw.tags["json"] = tw.jsonVal
	keys := make([]string, 0, len(tw.tags))
	for tag := range tw.tags {
		keys = append(keys, tag)
	}
	// tags are arranged in alphabet order.
	sort.Strings(keys)
	for _, tag := range keys {
		tagVal := tw.tags[tag]
		if tag == "json" {
			sb.WriteString(` json:"` + tw.jsonVal + jsonSuffix + `"`)
			continue
		}
		if tw.opt.snakeStyleHTTPTag {
			tagVal = util.Snakify(tagVal)
		}
		sb.WriteString(` ` + tag + `:"` + tagVal + tagSuffix + `"`)
	}
	return sb.String()
}
