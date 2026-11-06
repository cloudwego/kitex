package remote_test

import "github.com/cloudwego/kitex/pkg/remote"

// Keep the original positional forms compiling. An API diff alone misses
// additions to these all-exported structs.
var (
	_ = remote.ConnOption{nil, 0}
	_ = remote.Option{nil, nil, nil}
)
