package dirfiles

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/boringcat/just-a-log-viewer/server"
	"github.com/nxadm/tail"
)

var (
	NEWLINE_LINUX   = []byte{'\n'}
	NEWLINE_MACOS   = []byte{'\r'}
	NEWLINE_WINDOWS = []byte{'\r', '\n'}
	ConfigFilePath  string
)

type File struct {
	Hash   string            `json:"hash"`
	Name   string            `json:"name"`
	Path   string            `json:"-"`
	Labels map[string]string `json:"labels"`
}

type Server struct {
	conf      *Config
	fmap      *sync.Map
	lastFetch time.Time
}

func NewServer() (server.LogServer, error) {
	if len(ConfigFilePath) == 0 {
		return nil, nil
	}
	confs, err := ReadConfig(ConfigFilePath)
	if err != nil {
		return nil, err
	}
	s := Server{conf: confs}
	go s.doGlobWalk()
	return &s, nil
}

func (s *Server) doGlobWalk() {
	if time.Since(s.lastFetch) < 10*time.Minute {
		return
	}
	s.lastFetch = time.Now()
	var newMap sync.Map
	for f := range DoGlobWalk(s.conf) {
		newMap.Store(f.Hash, f)
	}
	s.fmap = &newMap
}

func (s *Server) HandleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		server.HTTPError(w, http.StatusMethodNotAllowed)
		return
	}
	s.doGlobWalk()
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	fmt.Fprint(w, `{"keys":`)
	enc.Encode(s.conf.Keys)
	fmt.Fprint(w, `,"files":`)
	sep := "["
	s.fmap.Range(func(key, value any) bool {
		fmt.Fprint(w, sep)
		enc.Encode(value)
		sep = ","
		return true
	})
	if sep == "[" {
		fmt.Fprint(w, sep)
	}
	fmt.Fprint(w, "]}")
}

func (s *Server) getFile(h string) (string, error) {
	slog.Debug("查询文件", "hash", h)
	val, ok := s.fmap.Load(h)
	if !ok {
		return "", os.ErrNotExist
	}
	return val.(*File).Path, nil
}

func (s *Server) HandleTail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		server.HTTPError(w, http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	if err := server.EnsureKeys(q, "h"); err != nil {
		server.HTTPError(w, http.StatusBadRequest)
		return
	}
	fpath, err := s.getFile(q.Get("h"))
	if err != nil {
		server.HTTPError(w, http.StatusNotFound)
		return
	}

	var tail_ int64 = 1000
	if q.Has("tail") {
		if val, err := strconv.ParseInt(q.Get("tail"), 10, 64); err == nil {
			tail_ = val
		}
	}
	var contentType string
	for _, ext := range GetExts(fpath) {
		contentType = mime.TypeByExtension(ext)
		if len(contentType) > 0 {
			break
		}
	}
	if len(contentType) == 0 {
		contentType = "text/plain; charset=utf-8"
	}

	fd, err := os.Open(fpath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer fd.Close()
	var offset int64 = 0

	if tail_ > 0 {
		offset, err = GetTailOffset(fd, tail_)
		if err == io.EOF {
			http.NotFound(w, r)
			return
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fd.Seek(offset, io.SeekStart)
	}
	w.Header().Set("Content-Type", contentType)
	io.CopyBuffer(w, fd, make([]byte, server.GlobalBufSize))
}

func (s *Server) HandleWatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		server.HTTPError(w, http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	if err := server.EnsureKeys(q, "h"); err != nil {
		server.HTTPError(w, http.StatusBadRequest)
		return
	}
	fpath, err := s.getFile(q.Get("h"))
	if err != nil {
		server.HTTPError(w, http.StatusNotFound)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		server.HTTPError(w, http.StatusNotFound)
		return
	}

	var tail_ int64 = 1000
	if q.Has("tail") {
		if val, err := strconv.ParseInt(q.Get("tail"), 10, 64); err == nil {
			tail_ = val
		}
	}
	tc := tail.Config{Follow: true}
	if tail_ > 0 {
		offset, err := GetTailOffsetByFileName(fpath, tail_)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		tc.Location = &tail.SeekInfo{
			Offset: offset,
			Whence: io.SeekStart,
		}
	}
	t, err := tail.TailFile(fpath, tc)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer t.Stop()
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	for {
		select {
		case line := <-t.Lines:
			fmt.Fprintf(w, "data: %s\n\n", line.Text)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func init() {
	server.Register("dirfiles", NewServer)
}
