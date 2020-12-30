package routing

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
)

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (r *responseWriter) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (ro *routing) useBase(fn http.Handler) http.Handler {
	return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
		var (
			writer io.Writer
		)
		func() {
			if _, err := os.Stat(ro.logDir); os.IsNotExist(err) {
				if err = os.MkdirAll(ro.logDir, 0655); err != nil {
					writer = os.Stdout
					return
				}
				now := time.Now().Format("02-01-2006")
				logfile, err := os.OpenFile(filepath.Join(ro.logDir, now), os.O_APPEND|os.O_CREATE|os.O_RDWR, 655)
				if err != nil {
					writer = os.Stdout
					return
				}
				defer logfile.Close()
				writer = io.MultiWriter(os.Stdout, logfile)
			}
		}()
		rw := &responseWriter{
			status:         200,
			ResponseWriter: w,
		}
		fn.ServeHTTP(rw, r)
		fields := logrus.Fields{
			"method": r.Method,
			"status": rw.status,
		}
		ro.mu.Lock()
		defer ro.mu.Unlock()
		ro.logger.SetOutput(writer)
		ro.logger.WithFields(fields).Println(r.URL.Path)
		return nil
	})
}
