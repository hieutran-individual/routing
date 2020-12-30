package routing

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/sirupsen/logrus"
)

type routing struct {
	router         *mux.Router
	middlewares    []mux.MiddlewareFunc
	logDir         string
	logger         *logrus.Logger
	mu             sync.Mutex
	maxBytesReader *int64
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
	ReadSchema(r *http.Request, v interface{}) error
	WriteJSON(w http.ResponseWriter, v interface{})
	ReadJSON(r *http.Request, v interface{}) error
	SetLogDir(string)
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
	var (
		handler http.Handler = fn
	)
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i].Middleware(handler)
	}
	return handler
}

func (r *routing) Handle(path string, handler http.Handler) *mux.Route {
	return r.router.Handle(path, useMiddlewares(handler, r.middlewares...))
}

func (r *routing) HandleFunc(path string, handlerFunc func(http.ResponseWriter, *http.Request) error) *mux.Route {
	return r.router.HandleFunc(path, useMiddlewares(HandlerFunc(handlerFunc), r.middlewares...).ServeHTTP)
}

func (r *routing) Use(middlewares ...mux.MiddlewareFunc) {
	for _, mdw := range middlewares {
		r.middlewares = append(r.middlewares, mdw)
	}
}
func (h *routing) ReadJSON(r *http.Request, v interface{}) error {
	var (
		maxBytesReader int64
	)
	if h.maxBytesReader != nil {
		maxBytesReader = *h.maxBytesReader
	} else {
		maxBytesReader = 10 << 20
	}
	body := http.MaxBytesReader(nil, r.Body, maxBytesReader)
	return json.NewDecoder(body).Decode(v)
}

func (h *routing) WriteJSON(w http.ResponseWriter, v interface{}) {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (h *routing) ReadSchema(r *http.Request, v interface{}) error {
	return schema.NewDecoder().Decode(v, r.URL.Query())
}

func (h *routing) SetLogDir(path string) {
	h.logDir = path
}
