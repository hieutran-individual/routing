package routing

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

type logRoute struct {
	router      *mux.Router
	middlewares []mux.MiddlewareFunc
	logDir      string
	logger      *logrus.Logger
	mu          sync.Mutex
}

type LogFn func(fields logrus.Fields)

type HandlerFunc func(w http.ResponseWriter, r *http.Request, l LogFn) error

func (h HandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fields, ok := r.Context().Value(CtxLogFn).(logrus.Fields)
	if !ok {
		fields = logrus.Fields{}
		ctx := context.WithValue(r.Context(), CtxLogFn, fields)
		r = r.WithContext(ctx)
	}
	err := h(w, r, func(f logrus.Fields) {
		for k, v := range f {
			fields[k] = v
		}
	})
	if err != nil {
		fields["error"] = fmt.Sprintf("%+v", err)
	}
}

type LogRoute interface {
	Use(...mux.MiddlewareFunc)
	Handle(path string, handler http.Handler) *mux.Route
	HandleFunc(path string, handlerFunc http.HandlerFunc) *mux.Route
	DoHandler(path string, handlerFunc HandlerFunc) *mux.Route
	SetLogDir(string)
	Subrouter(path string) *logRoute
}

func New(r *mux.Router, pathPrefix string) LogRoute {
	return newLogRoute(r, pathPrefix)
}

func newLogRoute(r *mux.Router, pathPrefix string) *logRoute {
	workingDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("cannot write get working directory: %+v", err)
	}
	ro := &logRoute{router: r.PathPrefix(pathPrefix).Subrouter(), logDir: workingDir, logger: logrus.New()}
	ro.middlewares = []mux.MiddlewareFunc{ro.useLogging}
	ro.logger.SetFormatter(&logrus.JSONFormatter{
		PrettyPrint:     true,
		TimestampFormat: "02-01-2006 15:04:05",
	})
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

func (r *logRoute) Handle(path string, handler http.Handler) *mux.Route {
	return r.router.Handle(path, useMiddlewares(handler, r.middlewares...))
}

func (r *logRoute) HandleFunc(path string, handlerFunc http.HandlerFunc) *mux.Route {
	return r.router.HandleFunc(path, useMiddlewares(handlerFunc, r.middlewares...).ServeHTTP)
}

func (r *logRoute) DoHandler(path string, handlerFunc HandlerFunc) *mux.Route {
	return r.router.HandleFunc(path, useMiddlewares(HandlerFunc(handlerFunc), r.middlewares...).ServeHTTP)
}

func (r *logRoute) Use(middlewares ...mux.MiddlewareFunc) {
	for _, mdw := range middlewares {
		r.middlewares = append(r.middlewares, mdw)
	}
}

func (h *logRoute) SetLogDir(path string) {
	h.logDir = path
}

func (h *logRoute) Subrouter(path string) *logRoute {
	ro := &logRoute{
		router: h.router.PathPrefix(path).Subrouter(),
		logDir: h.logDir,
		logger: h.logger,
	}
	ro.Use(h.middlewares...)
	return ro
}
