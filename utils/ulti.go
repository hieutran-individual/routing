package utils

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/hieutran-individual/status"
	"github.com/hieutran-individual/status/codes"
	"github.com/hieutran-individual/status/pb"
)

type Utils struct {
	maxBytesReader *int64
}

func (h *Utils) SetMaxBytesReader(max int64) {
	h.maxBytesReader = &max
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

func (h *Utils) WriteJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Add("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

type protoGrpcResponse struct {
	*pb.Status
}

func (h *Utils) WriteJSONGrpc(w http.ResponseWriter, v interface{}, err error) {
	status, ok := status.FromError(err)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
	}
	if status.Code() != codes.OK {
		h.WriteJSON(w, &protoGrpcResponse{status.Proto()})
		return
	}
	h.WriteJSON(w, v)
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
