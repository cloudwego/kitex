namespace * parser

typedef list<Annotation> Annotations

enum Category {
    Constant
	Bool
    Byte // I8
    I16
    I32
    I64
    Double
    String
    Binary
    Map
    List
    Set
    Enum
    Struct
    Union
    Exception
    Typedef
    Service
}

struct Reference {
    1: string Name // The name of the referenced type with out IDL name prefix.
    2: i32 Index   // The index of the included IDL that contains the referenced type
}

struct Annotation {
    1: string Key
    2: list<string> Values
}

struct Type {
    1: string Name                     // base types | container types | identifier | selector
    2: optional Type KeyType           // if Name is 'map'
    3: optional Type ValueType         // if Name is 'map', 'list', or 'set'
    4: string CppType                  // map, set, list
    5: Annotations Annotations
    6: Category Category               // the **final** category resolved
    7: optional Reference Reference    // when Name is an identifier referring to an external type
    8: optional bool IsTypedef         // whether this type is a typedef
}

struct Namespace {
    1: string Language
    2: string Name
    3: Annotations Annotations
}

struct Typedef {
    1: optional Type Type
    2: string Alias
    3: Annotations Annotations
}

struct EnumValue {
    1: string Name
    2: i64 Value
    3: Annotations Annotations
}

struct Enum {
    1: string Name
    2: list<EnumValue> Values
    3: Annotations Annotations
}

enum ConstType {
    ConstDouble
    ConstInt
    ConstLiteral
    ConstIdentifier
    ConstList
    ConstMap
}

// ConstValueExtra provides extra information when the Type of a ConstValue is ConstIdentifier.
struct ConstValueExtra {
    1: bool IsEnum    // whether the value resolves to an enum value
    2: i32 Index = -1 // the include index if Index > -1
    3: string Name    // the name of the value without selector
    4: string Sel     // the selector
}

struct ConstValue {
    1: ConstType Type
    2: optional ConstTypedValue TypedValue
    3: optional ConstValueExtra Extra
}

union ConstTypedValue {
    1: double Double
    2: i64 Int
    3: string Literal
    4: string Identifier
    5: list<ConstValue> List
    6: list<MapConstValue> Map
}

struct MapConstValue {
    1: optional ConstValue Key
    2: optional ConstValue Value
}

struct Constant {
    1: string Name
    2: optional Type Type
    3: optional ConstValue Value
    4: Annotations Annotations
}

enum FieldType {
    Default
    Required
    Optional
}

struct Field {
    1: i32 ID
    2: string Name
    3: FieldType Requiredness
    4: Type Type
    5: optional ConstValue Default // ConstValue
    6: Annotations Annotations
}

struct StructLike {
    1: string Category // "struct", "union" or "exception"
    2: string Name
    3: list<Field> Fields
    4: Annotations Annotations
}

struct Function {
    1: string Name
    2: bool Oneway
    3: bool Void
    4: optional Type FunctionType
    5: list<Field> Arguments
    6: list<Field> Throws
    7: Annotations Annotations
}

struct Service {
    1: string Name
    2: string Extends
    3: list<Function> Functions
    4: Annotations Annotations

    // If Extends is not empty and it references to a service defined in an
    // included IDL, then Reference will be set.
    5: optional Reference Reference
}

struct Include {
    1: string Path               // The path literal in the include statement.
    2: optional Thrift Reference // The parsed AST of the included IDL.
    3: optional bool Used        // If this include is used in the IDL
}

// Thrift is the AST of the current IDL with symbols sorted.
struct Thrift {
    1: string Filename            // A valid path of current thrift IDL.
    2: list<Include> Includes     // Direct dependencies.
    3: list<string> CppIncludes
    4: list<Namespace> Namespaces
    5: list<Typedef> Typedefs
    6: list<Constant> Constants
    7: list<Enum> Enums
    8: list<StructLike> Structs
    9: list<StructLike> Unions
    10: list<StructLike> Exceptions
    11: list<Service> Services

    // Name2Category keeps a mapping for all global names with their **direct** category.
    12: map<string, Category> Name2Category
}
