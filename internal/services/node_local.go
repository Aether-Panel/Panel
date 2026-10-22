package services

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
)

// localBufferLimit is how much of a local node's response is kept in memory. Smaller
// answers are returned whole, so callers that never read the body leave nothing running;
// bigger ones (file downloads, backups) are streamed instead of being held in RAM.
const localBufferLimit = 1 << 20

// serveLocal runs the request against this process's own daemon routes
func serveLocal(request *http.Request) *http.Response {
	w := &localResponseWriter{
		header:  http.Header{},
		ready:   make(chan *http.Response, 1),
		request: request,
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logging.Error.Printf("Panic serving local node request %s: %v", request.URL.Path, r)
				if w.pipe != nil {
					_ = w.pipe.CloseWithError(fmt.Errorf("local node request failed: %v", r))
					return
				}
				w.status = http.StatusInternalServerError
				w.buffered.Reset()
			}
			w.finish()
		}()
		skypanel.Engine.ServeHTTP(w, request)
	}()

	return <-w.ready
}

type localResponseWriter struct {
	header   http.Header
	status   int
	buffered bytes.Buffer
	pipe     *io.PipeWriter
	ready    chan *http.Response
	request  *http.Request
}

func (w *localResponseWriter) Header() http.Header {
	return w.header
}

func (w *localResponseWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
}

func (w *localResponseWriter) Write(b []byte) (int, error) {
	w.WriteHeader(http.StatusOK)
	if w.pipe != nil {
		return w.pipe.Write(b)
	}
	if w.buffered.Len()+len(b) <= localBufferLimit {
		return w.buffered.Write(b)
	}

	// too big to keep in memory: hand the caller a stream and keep writing into it. The
	// buffer is no longer written to, so the caller can read it while the handler goes on.
	reader, writer := io.Pipe()
	w.pipe = writer
	w.send(&pipedBody{Reader: io.MultiReader(&w.buffered, reader), pipe: reader}, -1)
	return writer.Write(b)
}

// Flush is a no-op: data reaches the caller as soon as it reads
func (w *localResponseWriter) Flush() {}

func (w *localResponseWriter) finish() {
	if w.pipe != nil {
		_ = w.pipe.Close()
		return
	}
	w.WriteHeader(http.StatusOK)
	w.send(io.NopCloser(bytes.NewReader(w.buffered.Bytes())), int64(w.buffered.Len()))
}

func (w *localResponseWriter) send(body io.ReadCloser, length int64) {
	w.ready <- &http.Response{
		Status:        fmt.Sprintf("%d %s", w.status, http.StatusText(w.status)),
		StatusCode:    w.status,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        w.header.Clone(),
		Body:          body,
		ContentLength: length,
		Request:       w.request,
	}
}

// pipedBody lets the caller stop a streamed response early: closing it makes the handler's
// next write fail instead of blocking forever
type pipedBody struct {
	io.Reader
	pipe *io.PipeReader
}

func (b *pipedBody) Close() error {
	return b.pipe.Close()
}
