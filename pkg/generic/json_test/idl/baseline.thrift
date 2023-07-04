namespace go baseline

struct Simple {
    1: byte ByteField
    2: i64 I64Field
    3: double DoubleField
    4: i32 I32Field
    5: string StringField,
    6: binary BinaryField
}

struct Nesting {
    1: string String
    2: list<Simple> ListSimple
    3: double Double
    4: i32 I32
    5: list<i32> ListI32
    6: i64 I64
    7: map<string, string> MapStringString
    8: Simple SimpleStruct
    9: map<i32, i64> MapI32I64
    10: list<string> ListString
    11: binary Binary
    12: map<i64, string> MapI64String
    13: list<i64> ListI64
    14: byte Byte
    15: map<string, Simple> MapStringSimple
}

service BaselineService {
    Simple SimpleMethod(1: Simple req)
    Nesting NestingMethod(1: Nesting req)
}
