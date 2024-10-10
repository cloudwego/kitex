package ttstream

type tException interface {
	Error() string
	TypeId() int32
}
