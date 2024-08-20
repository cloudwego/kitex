package streamx

type StreamHandler struct {
	Middleware StreamMiddleware
	Handler    interface{}
}
