package httpdoc

type middlewareFunc func(*Document) error

var middlewares []middlewareFunc

func AddMiddleware(fn middlewareFunc) {
	middlewares = append(middlewares, fn)
}
