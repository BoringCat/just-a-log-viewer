package server

import (
	"io"
	"log/slog"
	"net/http"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/google/brotli/go/cbrotli"
	"github.com/klauspost/compress/flate"
	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zstd"
)

type WriteCloseFlusher interface {
	io.WriteCloser
	Flush() error
}

type compressResponseWriter struct {
	w WriteCloseFlusher
	http.ResponseWriter
}

func (w compressResponseWriter) Write(b []byte) (int, error) {
	return w.w.Write(b)
}

func (w compressResponseWriter) Flush() {
	w.w.Flush()
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func isSpaceAndComma(r rune) bool {
	return unicode.IsSymbol(r) || r == ','
}

type CompressHandler struct {
	next http.Handler
	opt  *CompressOpts
}

type CompressOpts struct {
	Order        []string
	GzipLevel    int
	DeflateLevel int
	ZstdLevel    int
	BrotilLevel  int
	zstdLevel    zstd.EncoderLevel
}

func (o *CompressOpts) Verify() {
	if len(o.Order) == 0 {
		o.Order = SupportedCompress
		slog.Warn("CompressOpts: Order值异常，已重置", "Order", SupportedCompress)
	}
	if o.GzipLevel < gzip.ConstantCompression || o.GzipLevel > gzip.BestCompression {
		o.GzipLevel = gzip.DefaultCompression
		slog.Warn("CompressOpts: GzipLevel值异常，已重置", "GzipLevel", gzip.DefaultCompression)
	}
	if o.DeflateLevel < flate.ConstantCompression || o.DeflateLevel > flate.BestCompression {
		o.DeflateLevel = flate.DefaultCompression
		slog.Warn("CompressOpts: DeflateLevel值异常，已重置", "DeflateLevel", flate.DefaultCompression)
	}
	if o.ZstdLevel < int(zstd.SpeedFastest) || o.ZstdLevel > int(zstd.SpeedBestCompression) {
		o.zstdLevel = zstd.SpeedDefault
		slog.Warn("CompressOpts: ZstdLevel值异常，已重置", "ZstdLevel", zstd.SpeedDefault)
	} else {
		o.zstdLevel = zstd.EncoderLevel(o.ZstdLevel)
	}
	if o.BrotilLevel < 0 || o.BrotilLevel > 11 {
		o.BrotilLevel = 6
		slog.Warn("CompressOpts: BrotilLevel值异常，已重置", "BrotilLevel", 6)
	}
}

var (
	SupportedCompress  = []string{"br", "zstd", "gzip", "deflate"}
	DefaultCompressOpt = &CompressOpts{
		Order:        SupportedCompress,
		GzipLevel:    gzip.DefaultCompression,
		DeflateLevel: flate.DefaultCompression,
		zstdLevel:    zstd.SpeedDefault,
		BrotilLevel:  6,
	}
)

func (h *CompressHandler) getOrderEncoding(enc string) []string {
	fields := []string{}
	for field := range strings.FieldsFuncSeq(enc, isSpaceAndComma) {
		fields = append(fields, strings.TrimFunc(field, unicode.IsSpace))
	}
	sort.Slice(fields, func(i, j int) bool {
		_i := slices.Index(h.opt.Order, fields[i])
		_j := slices.Index(h.opt.Order, fields[j])
		if _i < 0 {
			_i += 1024
		}
		if _j < 0 {
			_j += 1024
		}
		return _i < _j
	})
	return fields
}

func (h *CompressHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var rw WriteCloseFlusher
	var err error
	for _, accepted := range h.getOrderEncoding(r.Header.Get("Accept-Encoding")) {
		if rw != nil {
			break
		}
		switch accepted {
		case "gzip":
			if rw, err = gzip.NewWriterLevel(w, h.opt.GzipLevel); err != nil {
				slog.Warn("加载Gzip压缩器失败", "err", err)
				rw = nil
				continue
			}
			w.Header().Set("Content-Encoding", "gzip")
		case "deflate":
			if rw, err = flate.NewWriter(w, h.opt.DeflateLevel); err != nil {
				slog.Warn("加载Deflate压缩器失败", "err", err)
				rw = nil
				continue
			}
			w.Header().Set("Content-Encoding", "deflate")
		case "br":
			rw = cbrotli.NewWriter(w, cbrotli.WriterOptions{Quality: h.opt.BrotilLevel})
			w.Header().Set("Content-Encoding", "br")
		case "zstd":
			if rw, err = zstd.NewWriter(w, zstd.WithEncoderLevel(h.opt.zstdLevel)); err != nil {
				slog.Warn("加载Zstd压缩器失败", "err", err)
				rw = nil
				continue
			}
			w.Header().Set("Content-Encoding", "zstd")
		}
	}
	if rw == nil {
		h.next.ServeHTTP(w, r)
	} else {
		defer rw.Close()
		h.next.ServeHTTP(compressResponseWriter{w: rw, ResponseWriter: w}, r)
	}
}

func NewCompressHandler(next http.Handler, opt *CompressOpts) http.Handler {
	if opt == nil {
		opt = DefaultCompressOpt
	}
	return &CompressHandler{next: next, opt: opt}
}
