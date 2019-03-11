package server

import (
	"compress/gzip"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/xerrors"
)

var gzipPool = sync.Pool{
	New: func() interface{} {
		return gzip.NewWriter(ioutil.Discard)
	},
}

func enableGzip(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !shouldCompress(r) {
			handler.ServeHTTP(w, r)
			return
		}

		respRecorder := httptest.NewRecorder()

		handler.ServeHTTP(respRecorder, r)

		result := respRecorder.Result()

		if result.StatusCode != http.StatusOK {
			copyHeader(w.Header(), result.Header)
			w.WriteHeader(result.StatusCode)
			io.Copy(w, result.Body)
			return
		}

		gzipWriter := gzipPool.Get().(*gzip.Writer)
		gzipWriter.Reset(w)

		defer func() {
			gzipWriter.Close()
			gzipPool.Put(gzipWriter)
		}()

		copyHeader(w.Header(), result.Header)
		w.Header().Del("content-length")
		w.Header().Set("content-encoding", "gzip")

		if _, err := io.Copy(gzipWriter, result.Body); err != nil {
			log.Printf("%+v", xerrors.Errorf("compress http response failed: %w", err))
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
	})
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func shouldCompress(req *http.Request) bool {
	if !strings.Contains(req.Header.Get("Accept-Encoding"), "gzip") ||
		strings.Contains(req.Header.Get("Connection"), "Upgrade") ||
		strings.Contains(req.Header.Get("Content-Type"), "text/event-stream") {

		return false
	}

	extension := filepath.Ext(req.URL.Path)
	if len(extension) < 4 { // fast path
		return true
	}

	// log.Println("extension:", extension)

	switch extension {
	case ".png", ".gif", ".jpeg", ".jpg":
		return false
	default:
		return true
	}
}
