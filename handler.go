package routing

import (
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

type routing struct {
	router      *mux.Router
	middlewares []mux.MiddlewareFunc
	logDir      string
	logger      *logrus.Logger
	mu          sync.Mutex
}

// The HandlerFunc type is an adapter to allow the use of
// ordinary functions as HTTP handlers.
type HandlerFunc func(w http.ResponseWriter, r *http.Request) error

// ServeHTTP calls f(w, r).
func (h HandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h(w, r)
}

type Routing interface {
	Use(...mux.MiddlewareFunc)
	Handle(path string, handler http.Handler) *mux.Route
	HandleFunc(path string, handlerFunc func(http.ResponseWriter, *http.Request) error) *mux.Route
}

func New(r *mux.Router, pathPrefix string) Routing {
	workingDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("cannot write get working directory: %+v", err)
	}
	ro := &routing{router: r.PathPrefix(pathPrefix).Subrouter(), logDir: workingDir, logger: logrus.New()}
	ro.middlewares = []mux.MiddlewareFunc{ro.useBase}
	return ro
}

func useMiddlewares(fn http.Handler, middlewares ...mux.MiddlewareFunc) http.Handler {
	handler := fn
	for i := len(middlewares); i >= 0; i-- {
		handler = middlewares[i](fn)
	}
	return handler
}

func (r *routing) Handle(path string, handler http.Handler) *mux.Route {
	return r.router.Handle(path, useMiddlewares(handler, r.middlewares...))
}

func (r *routing) HandleFunc(path string, handlerFunc func(http.ResponseWriter, *http.Request) error) *mux.Route {
	return r.Handle(path, useMiddlewares(HandlerFunc(handlerFunc), r.middlewares...))
}

func (r *routing) Use(middlewares ...mux.MiddlewareFunc) {
	for _, mdw := range middlewares {
		r.middlewares = append(r.middlewares, mdw)
	}
}
