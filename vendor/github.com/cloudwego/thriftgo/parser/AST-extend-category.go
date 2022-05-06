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

package parser

// IsConstant tells if the category is constant.
func (p Category) IsConstant() bool {
	return p == Category_Constant
}

// IsBool tells if the category is bool.
func (p Category) IsBool() bool {
	return p == Category_Bool
}

// IsByte tells if the category is byte.
func (p Category) IsByte() bool {
	return p == Category_Byte
}

// IsI16 tells if the category is i16.
func (p Category) IsI16() bool {
	return p == Category_I16
}

// IsI32 tells if the category is i32.
func (p Category) IsI32() bool {
	return p == Category_I32
}

// IsI64 tells if the category is i64.
func (p Category) IsI64() bool {
	return p == Category_I64
}

// IsDouble tells if the category is double.
func (p Category) IsDouble() bool {
	return p == Category_Double
}

// IsString tells if the category is string.
func (p Category) IsString() bool {
	return p == Category_String
}

// IsBinary tells if the category is binary.
func (p Category) IsBinary() bool {
	return p == Category_Binary
}

// IsMap tells if the category is map.
func (p Category) IsMap() bool {
	return p == Category_Map
}

// IsList tells if the category is list.
func (p Category) IsList() bool {
	return p == Category_List
}

// IsSet tells if the category is set.
func (p Category) IsSet() bool {
	return p == Category_Set
}

// IsEnum tells if the category is enum.
func (p Category) IsEnum() bool {
	return p == Category_Enum
}

// IsStruct tells if the category is struct.
func (p Category) IsStruct() bool {
	return p == Category_Struct
}

// IsUnion tells if the category is union.
func (p Category) IsUnion() bool {
	return p == Category_Union
}

// IsException tells if the category is exception.
func (p Category) IsException() bool {
	return p == Category_Exception
}

// IsTypedef tells if the category is typedef.
func (p Category) IsTypedef() bool {
	return p == Category_Typedef
}

// IsService tells if the category is service.
func (p Category) IsService() bool {
	return p == Category_Service
}

// IsBaseType tells if the category is one of the basetypes.
func (p Category) IsBaseType() bool {
	return int64(Category_Bool) <= int64(p) && int64(p) <= int64(Category_Binary)
}

// IsContainerType tells if the category is one of the container types.
func (p Category) IsContainerType() bool {
	return p == Category_Map || p == Category_Set || p == Category_List
}

// IsStructLike tells if the category is a struct-like.
func (p Category) IsStructLike() bool {
	return p == Category_Struct || p == Category_Union || p == Category_Exception
}
