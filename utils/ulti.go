package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/hieutran-individual/routing/codes"
	"github.com/hieutran-individual/routing/pb"
	"github.com/hieutran-individual/routing/status"
)

type Utils struct {
	maxBytesReader *int64
	logger         Logger
}

func (h *Utils) SetMaxBytesReader(max int64) {
	h.maxBytesReader = &max
}

type Logger interface {
	WriteLog(format string, args ...interface{})
}

func (h *Utils) SetLogger(l Logger) {
	h.logger = l
}

func (h *Utils) writeLog(format string, args ...interface{}) {
	if h.logger == nil {
		fmt.Printf(format, args...)
		return
	}
	fmt.Printf(format, args...)
}

func (h *Utils) ReadJSON(r *http.Request, v interface{}) error {
	var (
		maxBytesReader int64
	)
	if !strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		return errors.New("content-type is not application/json")
	}
	if h.maxBytesReader != nil {
		maxBytesReader = *h.maxBytesReader
	} else {
		maxBytesReader = 10 << 20
	}
	body := http.MaxBytesReader(nil, r.Body, maxBytesReader)
	return json.NewDecoder(body).Decode(v)
}

type ProblemJSON struct {
	*pb.Status
	Instance string `json:"instance"`
}

func (h *Utils) WriteJSON(w http.ResponseWriter, r *http.Request, v interface{}, err error) {
	stt, ok := status.FromError(err)
	if !ok {
		h.writeLog("this is not the standard status error: %+v", err)
		h.WriteJSON(w, r, v, status.Err(codes.Unknown, err.Error()))
		return
	}
	if stt == nil {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(v); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			h.writeLog("cannot encode response json: %v", err)
		}
		return
	}
	w.Header().Set("Content-Type", "application/problem+json")
	if err := json.NewEncoder(w).Encode(&ProblemJSON{stt.Proto(), r.URL.Path}); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		h.writeLog("cannot encode problem json: %v", err)
	}
	return
}

func (h *Utils) ReadSchema(r *http.Request, v interface{}) error {
	return schema.NewDecoder().Decode(v, r.URL.Query())
}

func (h *Utils) ParseUrlVars(r *http.Request, v interface{}) error {
	vars := mux.Vars(r)
	buf, err := json.Marshal(vars)
	if err != nil {
		return err
	}
	return json.Unmarshal(buf, v)
}
