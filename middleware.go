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
		rw := &responseWriter{
			status:         200,
			ResponseWriter: w,
		}
		fn.ServeHTTP(rw, r)
		fields := logrus.Fields{
			"method": r.Method,
			"status": rw.status,
		}
		ro.writeLog(r.URL.Path, fields)
		return nil
	})
}

func (ro *routing) writeLog(path string, fields logrus.Fields) {
	writer, logCloser := func() (io.Writer, io.Closer) {
		if _, err := os.Stat(ro.logDir); os.IsNotExist(err) {
			if err = os.MkdirAll(ro.logDir, 0655); err != nil {
				return os.Stdout, nil
			}
		}
		now := time.Now().Format("02-01-2006")
		logfile, err := os.OpenFile(filepath.Join(ro.logDir, now+".log"), os.O_APPEND|os.O_CREATE|os.O_RDWR, 0655)
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
