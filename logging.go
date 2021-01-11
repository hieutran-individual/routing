package routing

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type ResponseWriter struct {
	http.ResponseWriter
	buff   *bytes.Buffer
	status int
}

func (r *ResponseWriter) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *ResponseWriter) Header() http.Header {
	return r.ResponseWriter.Header()
}

func (r *ResponseWriter) Write(body []byte) (int, error) {
	contentType := http.DetectContentType(body)
	if !strings.Contains(contentType, "text/plain") && !strings.Contains(contentType, "application/json") {
		return r.ResponseWriter.Write(body)
	}
	if len(body) >= 2<<20 {
		r.buff = nil
	} else {
		fmt.Println("enter")
		r.buff = &bytes.Buffer{}
		writer := io.MultiWriter(r.ResponseWriter, r.buff)
		return writer.Write(body)
	}
	return r.ResponseWriter.Write(body)
}

func (ro *logRoute) useLogging(fn http.Handler) http.Handler {
	return HandlerFunc(func(w http.ResponseWriter, r *http.Request, l LogFn) error {
		rw := &ResponseWriter{
			status:         200,
			ResponseWriter: w,
		}
		logRequest := logrus.Fields{
			"method":         r.Method,
			"remote":         r.RemoteAddr,
			"user-agent":     r.UserAgent(),
			"content-length": r.ContentLength,
			"content-type":   r.Header.Get("Content-Type"),
			"request-uri":    r.RequestURI,
			"referer":        r.Referer(),
		}
		if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
			body := logrus.Fields{}
			if err := ro.ReadJSON(r, &body); err != nil {
				logRequest["body"] = errors.WithMessage(err, "cannot decode response body")
			} else {
				logRequest["body"] = body
			}
		}
		t := time.Now()
		fn.ServeHTTP(rw, r)
		since := time.Since(t).Milliseconds()
		fields, ok := r.Context().Value(CtxLogFn).(logrus.Fields)
		if !ok {
			return nil
		}
		fields["http/request"] = logRequest
		logResponse := logrus.Fields{
			"status":       rw.status,
			"content-type": rw.Header().Get("Content-Type"),
		}
		if rw.buff != nil {
			body := logrus.Fields{}
			if err := json.NewDecoder(rw.buff).Decode(&body); err != nil {
				logResponse["body"] = errors.WithMessage(err, "cannot decode response body")
			} else {
				logResponse["body"] = body
			}
		}
		fields["http/response"] = logResponse
		ro.writeLog(fmt.Sprintf("handled api took %d (ms)", since), fields)
		return nil
	})
}

func (ro *logRoute) writeLog(path string, fields logrus.Fields) {
	writer, logCloser := func() (io.Writer, io.Closer) {
		if _, err := os.Stat(ro.logDir); os.IsNotExist(err) {
			if err = os.MkdirAll(ro.logDir, 0755); err != nil {
				return os.Stdout, nil
			}
		}
		now := time.Now().Format("02-01-2006")
		logfile, err := os.OpenFile(filepath.Join(ro.logDir, now+".log"), os.O_APPEND|os.O_CREATE|os.O_RDWR, 0755)
		if err != nil {
			return os.Stdout, nil
		}
		return io.MultiWriter(os.Stdout, logfile), logfile
	}()
	ro.mu.Lock()
	defer ro.mu.Unlock()
	defer func() {
		if logCloser != nil {
			logCloser.Close()
		}
	}()
	ro.logger.SetOutput(writer)
	ro.logger.WithFields(fields).Println(path)
}
