package routing

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type logRoute struct {
	router         *mux.Router
	middlewares    []mux.MiddlewareFunc
	logDir         string
	logger         *logrus.Logger
	mu             sync.Mutex
	maxBytesReader *int64
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
	fmt.Println("ok")
	err := h(w, r, func(f logrus.Fields) {
		fmt.Println("enter")
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
	ReadSchema(r *http.Request, v interface{}) error
	WriteJSON(w http.ResponseWriter, v interface{})
	ReadJSON(r *http.Request, v interface{}) error
	WriteJSONGrpc(w http.ResponseWriter, v interface{}, err error)
	SetLogDir(string)
}

func New(r *mux.Router, pathPrefix string) LogRoute {
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

func (h *logRoute) ReadJSON(r *http.Request, v interface{}) error {
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

func (h *logRoute) WriteJSON(w http.ResponseWriter, v interface{}) {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

type ResponseWithStatus struct {
	*status.Status
}

func (h *logRoute) WriteJSONGrpc(w http.ResponseWriter, v interface{}, err error) {
	status, ok := status.FromError(err)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
	}
	if status.Code() != codes.OK {
		h.WriteJSON(w, &ResponseWithStatus{status})
		return
	}
	h.WriteJSON(w, v)
}

func (h *logRoute) ReadSchema(r *http.Request, v interface{}) error {
	return schema.NewDecoder().Decode(v, r.URL.Query())
}

func (h *logRoute) SetLogDir(path string) {
	h.logDir = path
}
