package dirfiles

import (
	"bytes"
	"fmt"
	"io"
	"iter"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/boringcat/just-a-log-viewer/server"
)

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

func GetTailOffset(r io.ReadSeeker, lines int64) (int64, error) {
	offset, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, err
	} else if offset < 2 {
		return 0, io.EOF
	}
	var count int64 = 0
	r.Seek(-2, io.SeekEnd)
	buf := make([]byte, server.GlobalBufSize)
	nr, err := r.Read(buf[0:2])
	if err != nil {
		return 0, err
	}
	if bytes.HasSuffix(buf[0:nr], NEWLINE_LINUX) || bytes.HasSuffix(buf[0:nr], NEWLINE_MACOS) || bytes.HasSuffix(buf[0:nr], NEWLINE_WINDOWS) {
		count--
	}
	bufsize := int64(server.GlobalBufSize)
	for count < lines && offset > 0 {
		if offset < bufsize {
			bufsize = offset
		}
		if _, err = r.Seek(-bufsize, io.SeekCurrent); err != nil {
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

func GetTailOffsetByFileName(fp string, lines int64) (int64, error) {
	fd, err := os.Open(fp)
	if err != nil {
		return 0, err
	}
	defer fd.Close()
	return GetTailOffset(fd, lines)
}

func DoGlobWalk(confs DirFileConfigs) iter.Seq[*File] {
	return func(yield func(*File) bool) {
		for cidx, conf := range confs {
			maxId := max(conf.KeyId, conf.NameId)
			for pidx, path := range conf.Paths {
				files, err := filepath.Glob(path)
				if err != nil {
					slog.Error("遍历文件列表失败", "err", err, "glob", path, "cidx", cidx, "pidx", pidx)
					continue
				}
				slog.Debug("遍历文件列表", "files", files, "glob", path, "cidx", cidx, "pidx", pidx)
				for _, file := range files {
					subm := conf.Regex.FindStringSubmatch(file)
					if len(subm) < maxId {
						slog.Error("正则表达式匹配失败", "file", file, "regex", conf.Regex.String(), "cidx", cidx, "pidx", pidx)
						continue
					}
					ok := yield(&File{
						Key:  subm[conf.KeyId],
						Name: subm[conf.NameId],
						Path: file,
					})
					if !ok {
						return
					}
				}
			}
		}
	}
}
