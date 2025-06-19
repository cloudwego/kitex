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
	"testing"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/thriftgo/parser"
)

func TestGenHTTPTags(t *testing.T) {
	emptyOpt := genHTTPTagOption{}
	// required, no annotation, generate form and query by default
	httpTag := genHTTPTags(&parser.Field{
		Name:         "f1",
		Requiredness: parser.FieldType_Required,
	}, emptyOpt)
	test.Assert(t, httpTag == ` form:"f1,required" json:"f1,required" query:"f1,required"`, httpTag)

	// default, no annotation
	httpTag = genHTTPTags(&parser.Field{
		Name:         "f11",
		Requiredness: parser.FieldType_Default,
	}, emptyOpt)
	test.Assert(t, httpTag == ` form:"f11" json:"f11" query:"f11"`, httpTag)

	// optional + go.tag (ignored by genHTTPTags)
	httpTag = genHTTPTags(&parser.Field{
		Name:         "f12",
		Requiredness: parser.FieldType_Optional,
		Annotations: []*parser.Annotation{
			{Key: "go.tag", Values: []string{`some_tag:"some_tag_value"`}},
		},
	}, emptyOpt)
	test.Assert(t, httpTag == ` form:"f12" json:"f12,omitempty" query:"f12"`, httpTag)

	// required, api.query
	httpTag = genHTTPTags(&parser.Field{
		Name:         "f22",
		Requiredness: parser.FieldType_Required,
		Annotations: []*parser.Annotation{
			{Key: "api.query", Values: []string{"abc"}},
		},
	}, emptyOpt)
	test.Assert(t, httpTag == ` json:"f22,required" query:"abc,required"`, httpTag)

	// required, api.form
	httpTag = genHTTPTags(&parser.Field{
		Name:         "f23",
		Requiredness: parser.FieldType_Required,
		Annotations: []*parser.Annotation{
			{Key: "api.form", Values: []string{"abc"}},
		},
	}, emptyOpt)
	test.Assert(t, httpTag == ` form:"abc,required" json:"f23,required"`, httpTag)

	// required, api.path
	httpTag = genHTTPTags(&parser.Field{
		Name:         "f24",
		Requiredness: parser.FieldType_Required,
		Annotations: []*parser.Annotation{
			{Key: "api.path", Values: []string{"abc"}},
		},
	}, emptyOpt)
	test.Assert(t, httpTag == ` json:"f24,required" path:"abc,required"`, httpTag)

	// required, api.header
	httpTag = genHTTPTags(&parser.Field{
		Name:         "f25",
		Requiredness: parser.FieldType_Required,
		Annotations: []*parser.Annotation{
			{Key: "api.header", Values: []string{"abc"}},
		},
	}, emptyOpt)
	test.Assert(t, httpTag == ` header:"abc,required" json:"f25,required"`, httpTag)

	// required, api.cookie
	httpTag = genHTTPTags(&parser.Field{
		Name:         "f26",
		Requiredness: parser.FieldType_Required,
		Annotations: []*parser.Annotation{
			{Key: "api.cookie", Values: []string{"abc"}},
		},
	}, emptyOpt)
	test.Assert(t, httpTag == ` cookie:"abc,required" json:"f26,required"`, httpTag)

	// required, api.raw_body
	httpTag = genHTTPTags(&parser.Field{
		Name:         "f27",
		Requiredness: parser.FieldType_Required,
		Annotations: []*parser.Annotation{
			{Key: "api.raw_body", Values: []string{"abc"}},
		},
	}, emptyOpt)
	test.Assert(t, httpTag == ` json:"f27,required" raw_body:"abc,required"`, httpTag)

	// required, multiple annotations, generate all
	httpTag = genHTTPTags(&parser.Field{
		Name:         "f31",
		Requiredness: parser.FieldType_Required,
		Annotations: []*parser.Annotation{
			{Key: "api.query", Values: []string{"abc"}},
			{Key: "api.form", Values: []string{"def"}},
			{Key: "api.header", Values: []string{"abc"}},
		},
	}, emptyOpt)
	// tags are arranged in alphabet order.
	test.Assert(t, httpTag == ` form:"def,required" header:"abc,required" json:"f31,required" query:"abc,required"`, httpTag)

	// required, duplicate api.query values, use the last one.
	httpTag = genHTTPTags(&parser.Field{
		Name:         "f32",
		Requiredness: parser.FieldType_Required,
		Annotations: []*parser.Annotation{
			{Key: "api.query", Values: []string{"abc", "def"}},
		},
	}, emptyOpt)
	test.Assert(t, httpTag == ` json:"f32,required" query:"def,required"`, httpTag)

	// required, api.body, api body generate json and form
	httpTag = genHTTPTags(&parser.Field{
		Name:         "f41",
		Requiredness: parser.FieldType_Required,
		Annotations: []*parser.Annotation{
			{Key: "api.body", Values: []string{"abc"}},
		},
	}, emptyOpt)
	test.Assert(t, httpTag == ` form:"abc,required" json:"abc,required"`, httpTag)

	// default, api.body
	httpTag = genHTTPTags(&parser.Field{
		Name:         "f42",
		Requiredness: parser.FieldType_Default,
		Annotations: []*parser.Annotation{
			{Key: "api.body", Values: []string{"abc"}},
		},
	}, emptyOpt)
	test.Assert(t, httpTag == ` form:"abc" json:"abc"`, httpTag)

	// default, api.body
	httpTag = genHTTPTags(&parser.Field{
		Name:         "f43",
		Requiredness: parser.FieldType_Optional,
		Annotations: []*parser.Annotation{
			{Key: "api.body", Values: []string{"abc"}},
		},
	}, emptyOpt)
	test.Assert(t, httpTag == ` form:"abc" json:"abc,omitempty"`, httpTag)

	// default, api.body, api.cookie
	httpTag = genHTTPTags(&parser.Field{
		Name:         "f43",
		Requiredness: parser.FieldType_Optional,
		Annotations: []*parser.Annotation{
			{Key: "api.body", Values: []string{"abc"}},
			{Key: "api.cookie", Values: []string{"efg"}},
		},
	}, emptyOpt)
	test.Assert(t, httpTag == ` cookie:"efg" form:"abc" json:"abc,omitempty"`, httpTag)
}

func TestGenHTTPTagsWithOption(t *testing.T) {
	// snake field name to camel, only for json
	httpTag := genHTTPTags(&parser.Field{
		Name:         "field_a",
		Requiredness: parser.FieldType_Required,
	}, genHTTPTagOption{lowerCamelCaseJSONTag: true})
	test.Assert(t, httpTag == ` form:"field_a,required" json:"fieldA,required" query:"field_a,required"`, httpTag)

	// camel field name to snake, only for json
	httpTag = genHTTPTags(&parser.Field{
		Name:         "FieldB",
		Requiredness: parser.FieldType_Required,
	}, genHTTPTagOption{snakeTyleJSONTag: true})
	test.Assert(t, httpTag == ` form:"FieldB,required" json:"field_b,required" query:"FieldB,required"`, httpTag)

	// snake opt + camel opt => snake fmt and then camel fmt
	httpTag = genHTTPTags(&parser.Field{
		Name:         "FieldB",
		Requiredness: parser.FieldType_Required,
	}, genHTTPTagOption{snakeTyleJSONTag: true, lowerCamelCaseJSONTag: true})
	test.Assert(t, httpTag == ` form:"FieldB,required" json:"fieldB,required" query:"FieldB,required"`, httpTag)

	// it also works for api.body json tag
	httpTag = genHTTPTags(&parser.Field{
		Name:         "field_c",
		Requiredness: parser.FieldType_Optional,
		Annotations: []*parser.Annotation{
			{Key: "api.body", Values: []string{"hey_hello"}},
		},
	}, genHTTPTagOption{lowerCamelCaseJSONTag: true})
	test.Assert(t, httpTag == ` form:"hey_hello" json:"heyHello,omitempty"`, httpTag)

	// snakeStyleHTTPTag option only works for form、query..... not for json and “json generated by api.body”
	httpTag = genHTTPTags(&parser.Field{
		Name:         "field_c",
		Requiredness: parser.FieldType_Optional,
		Annotations: []*parser.Annotation{
			{Key: "api.body", Values: []string{"heyHello"}},
			{Key: "api.query", Values: []string{"howAreYou"}},
			{Key: "api.cookie", Values: []string{"fineThanks"}},
		},
	}, genHTTPTagOption{snakeStyleHTTPTag: true})
	test.Assert(t, httpTag == ` cookie:"fine_thanks" form:"hey_hello" json:"heyHello,omitempty" query:"how_are_you"`, httpTag)

	// unsetOmitempty remove 'omitempty' in json
	httpTag = genHTTPTags(&parser.Field{
		Name:         "field_c",
		Requiredness: parser.FieldType_Optional,
		Annotations: []*parser.Annotation{
			{Key: "api.body", Values: []string{"heyHello"}},
			{Key: "api.query", Values: []string{"howAreYou"}},
			{Key: "api.cookie", Values: []string{"fineThanks"}},
		},
	}, genHTTPTagOption{unsetOmitempty: true})
	test.Assert(t, httpTag == ` cookie:"fineThanks" form:"heyHello" json:"heyHello" query:"howAreYou"`, httpTag)
}
