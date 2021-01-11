package routing

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
)

type ResponseWriter struct {
	http.ResponseWriter
	status int
}

func (r *ResponseWriter) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *ResponseWriter) Header() http.Header {
	return r.ResponseWriter.Header()
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
