package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/nxadm/tail"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

var (
	ErrUnsupportFormat = errors.New("unsupported format")
	NEWLINE_LINUX      = []byte{'\n'}
	NEWLINE_MACOS      = []byte{'\r'}
	NEWLINE_WINDOWS    = []byte{'\r', '\n'}
)

type File struct {
	Root string `json:"root"`
	File string `json:"file"`
	Name string `json:"name"`
}

type CRegexp struct {
	*regexp.Regexp
}

func (c *CRegexp) UnmarshalText(text []byte) error {
	re, err := regexp.Compile(string(text))
	if err != nil {
		return err
	}
	c.Regexp = re
	return nil
}

type DirFileConfig struct {
	Root  string   `json:"root" yaml:"root"`
	Regex *CRegexp `json:"regex" yaml:"regex"`
	KeyId int      `json:"keyid" yaml:"keyid"`
	hash  string
}

func (c *DirFileConfig) Prepare() {
	h := md5.New()
	fmt.Fprint(h, c.Root)
	c.hash = hex.EncodeToString(h.Sum(nil))
	if c.KeyId < 1 {
		c.KeyId = 1
	}
}

type DirFileConfigs []*DirFileConfig

func ReadConfig(filename string) (confs DirFileConfigs, err error) {
	if strings.HasSuffix(filename, ".yaml") || strings.HasSuffix(filename, ".yml") {
		fd, err := os.Open(filename)
		if err != nil {
			panic(err)
		}
		defer fd.Close()
		confs = DirFileConfigs{}
		if err = yaml.NewDecoder(fd).Decode(&confs); err != nil {
			return nil, err
		}
	} else if strings.HasSuffix(filename, ".json") {
		fd, err := os.Open(filename)
		if err != nil {
			panic(err)
		}
		defer fd.Close()
		confs = DirFileConfigs{}
		if err = json.NewDecoder(fd).Decode(&confs); err != nil {
			return nil, err
		}
	}
	if confs != nil {
		for _, conf := range confs {
			conf.Prepare()
		}
		return confs, nil
	}
	return nil, errors.Wrap(ErrUnsupportFormat, filename)
}

func GetTailOffset(r io.ReadSeeker, lines int) (int64, error) {
	offset, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, err
	} else if offset < 2 {
		return 0, io.EOF
	}
	count := 0
	r.Seek(-2, io.SeekEnd)
	buf := make([]byte, G_bufsize)
	nr, err := r.Read(buf[0:2])
	if err != nil {
		return 0, err
	}
	if bytes.HasSuffix(buf[0:nr], NEWLINE_LINUX) || bytes.HasSuffix(buf[0:nr], NEWLINE_MACOS) || bytes.HasSuffix(buf[0:nr], NEWLINE_WINDOWS) {
		count--
	}
	bufsize := int64(G_bufsize)
	for count < lines && offset > 0 {
		if offset < bufsize {
			bufsize = offset
		}
		if offset, err = r.Seek(-bufsize, io.SeekCurrent); err != nil {
			return 0, err
		}
		if nr, err = r.Read(buf[0:bufsize]); err != nil {
			return 0, err
		}
		slices.Reverse(buf[0:nr])
		offset, err = r.Seek(-int64(nr), io.SeekCurrent)
		if err != nil {
			return 0, err
		}
		thisoff := 0
		rd := bytes.NewReader(buf[0:nr])
		for r, s, err := rd.ReadRune(); err == nil; r, s, err = rd.ReadRune() {
			switch r {
			case '\r', '\n':
				count++
				if count >= lines {
					thisoff, _ := rd.Seek(0, io.SeekCurrent)
					return offset + int64(nr) - thisoff + int64(s), nil
				}
			}
			thisoff += s
		}
	}
	return 0, nil
}

func GetTailOffsetByFileName(fp string, lines int) (int64, error) {
	fd, err := os.Open(fp)
	if err != nil {
		return 0, err
	}
	defer fd.Close()
	return GetTailOffset(fd, lines)
}

func GetExts(name string) []string {
	resp := []string{}
	_old_ext := ""
	for {
		ext := filepath.Ext(name)
		if len(ext) == 0 {
			break
		}
		name = strings.TrimSuffix(name, ext)
		_old_ext = fmt.Sprint(_old_ext, ext)
		resp = append(resp, _old_ext)
	}
	return resp
}

func (c DirFileConfigs) HandleList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	sep := "["
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	for _, conf := range c {
		if err := filepath.Walk(conf.Root, func(path string, info fs.FileInfo, err error) error {
			if info.IsDir() {
				return nil
			}
			name := strings.TrimPrefix(strings.TrimPrefix(path, conf.Root), "/")
			subm := conf.Regex.FindStringSubmatch(path)
			if len(subm) < conf.KeyId {
				slog.Error("regex error", "path", path, "regex", conf.Regex.String())
				return nil
			}
			fmt.Fprint(w, sep)
			enc.Encode(&File{
				Root: conf.hash,
				File: name,
				Name: subm[conf.KeyId],
			})
			sep = ","
			return nil
		}); err != nil {
			slog.Error("walk dir error", "error", err, "dir", conf.Root)
			return
		}
	}
	fmt.Fprint(w, "]")
}

func (c DirFileConfigs) HandleTail(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if err := EnsureKeys(q, "root", "name"); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	root_hash := q.Get("root")
	var conf *DirFileConfig
	for _, _conf := range c {
		if _conf.hash == root_hash {
			conf = _conf
		}
	}
	if conf == nil {
		http.NotFound(w, r)
		return
	}

	var tail_ int64 = 1000
	if q.Has("tail") {
		if val, err := strconv.ParseInt(q.Get("tail"), 10, 64); err == nil {
			tail_ = val
		}
	}
	fp := filepath.Join(conf.Root, q.Get("name"))
	var contentType string
	for _, ext := range GetExts(fp) {
		contentType = mime.TypeByExtension(ext)
		if len(contentType) > 0 {
			break
		}
	}
	if len(contentType) == 0 {
		contentType = "text/plain; charset=utf-8"
	}

	fd, err := os.Open(fp)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer fd.Close()
	var offset int64 = 0

	if tail_ > 0 {
		offset, err = GetTailOffset(fd, int(tail_))
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
	io.CopyBuffer(w, fd, make([]byte, G_bufsize))
}

func (c DirFileConfigs) HandleWatch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if err := EnsureKeys(q, "root", "name"); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	root_hash := q.Get("root")
	var conf *DirFileConfig
	for _, _conf := range c {
		if _conf.hash == root_hash {
			conf = _conf
		}
	}
	if conf == nil {
		http.NotFound(w, r)
		return
	}

	var tail_ int64 = 1000
	if q.Has("tail") {
		if val, err := strconv.ParseInt(q.Get("tail"), 10, 64); err == nil {
			tail_ = val
		}
	}

	fp := filepath.Join(conf.Root, q.Get("name"))
	tc := tail.Config{Follow: true}
	if tail_ > 0 {
		offset, err := GetTailOffsetByFileName(fp, int(tail_))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		tc.Location = &tail.SeekInfo{Offset: offset, Whence: io.SeekStart}
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.NotFound(w, r)
		return
	}

	t, err := tail.TailFile(fp, tc)
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
