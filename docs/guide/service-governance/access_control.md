# Customized Access Control

The Kitex framework provides a middleware builder to support adding customized access control that rejects requests under certain conditions. The belowing is a simple example that randomly rejects 1% of all requests:

```go
package myaccesscontrol

import (
    "math/rand"
    "github.com/jackedelic/kitex/internal/pkg/acl"
)

var errRejected = errors.New("1% rejected")

// Implements a judge function.
func reject1percent(ctx context.Context, request interface{}) (reason error) {
    if rand.Intn(100) == 0 {
        return errRejected // an error should be returned when a request is rejected
    }
    return nil
}

var MyMiddleware = acl.NewACLMiddleware(reject1percent) // create the middleware
```

Then, you can enable this middleware with `WithMiddleware(myaccesscontrol.MyMiddleware)` at the creation of a client or a server.
