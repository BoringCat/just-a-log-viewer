//go:build linux && cgo

package journald

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os/exec"
	"sort"
	"strconv"
	"time"

	"github.com/boringcat/just-a-log-viewer/server"
	"github.com/coreos/go-systemd/sdjournal"
)

type Unit struct {
	Name        string `json:"unit"`
	Load        string `json:"load"`
	Active      string `json:"active"`
	State       string `json:"sub"`
	Description string `json:"description"`
}

type Units []*Unit

func (a Units) Len() int           { return len(a) }
func (a Units) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a Units) Less(i, j int) bool { return a[i].Name < a[j].Name }

type Server struct {
	services  Units
	lastFetch time.Time
}

func NewServer() (server.LogServer, error) {
	if Enabled {
		s := Server{
			services: Units{},
		}
		go s.getUnits()
		return &s, nil
	}
	return nil, nil
}

func (s *Server) getUnits() (Units, error) {
	if time.Since(s.lastFetch) < 10*time.Minute {
		slog.Debug("从缓存获取Systemd Units")
		return s.services, nil
	}
	slog.Debug("更新Systemd Units")
	s.lastFetch = time.Now()
	units := Units{}
	arg := []string{"systemctl", "list-units", "-o", "json", "--all"}
	if len(SystemdUnitState) > 0 {
		arg = append(arg, fmt.Sprintf("--state=%s", SystemdUnitState))
	}
	p := exec.Command("/usr/bin/env", arg...)
	out, err := p.StdoutPipe()
	if err != nil {
		return nil, err
	}
	defer out.Close()
	if err := p.Start(); err != nil {
		return nil, err
	}
	if err := json.NewDecoder(out).Decode(&units); err != nil {
		return nil, err
	}
	if err := p.Wait(); err != nil {
		return nil, err
	}
	sort.Sort(units)
	s.services = units
	return s.services, nil
}

func (s *Server) HandleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		server.HTTPError(w, http.StatusMethodNotAllowed)
		return
	}
	units, err := s.getUnits()
	if err != nil {
		slog.Error("获取Systemd Units异常", "err", err)
		server.HTTPError(w, http.StatusInternalServerError)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	sep := "["
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	for _, unit := range units {
		fmt.Fprint(w, sep)
		enc.Encode(unit.Name)
		sep = ","
	}
	fmt.Fprint(w, sep)
	enc.Encode("dmesg")
	fmt.Fprint(w, "]")
}

type Message struct {
	TimeStamp float64 `json:"ts"`
	Monotonic float64 `json:"monotonic"`
	HostName  string  `json:"hostname"`
	Process   string  `json:"process"`
	Pid       string  `json:"pid"`
	Message   string  `json:"message"`
	Priority  string  `json:"priority"`
}

func GetHttpSystemdJournal(q url.Values) (j *sdjournal.Journal, tail uint64, until time.Time, err error) {
	name := q.Get("name")
	j, err = sdjournal.NewJournal()
	if err != nil {
		return
	}
	if name == "kernel" || name == "dmesg" {
		j.AddMatch((&sdjournal.Match{Field: "_TRANSPORT", Value: "kernel"}).String())
	} else {
		if err = j.AddMatch((&sdjournal.Match{Field: "_SYSTEMD_UNIT", Value: name}).String()); err != nil {
			return
		}
		if err = j.AddDisjunction(); err != nil {
			return
		}
		if err = j.AddMatch((&sdjournal.Match{Field: "UNIT", Value: name}).String()); err != nil {
			return
		}
	}
	tail = 1000
	until = time.Unix(253402300799, 0)
	if q.Has("tail") {
		if val, err := strconv.ParseUint(q.Get("tail"), 10, 64); err == nil {
			tail = val
		}
	}
	if q.Has("until") {
		if val, err := strconv.ParseInt(q.Get("until"), 10, 64); err == nil {
			until = time.UnixMilli(val)
		}
	}
	return
}

func (s *Server) HandleTail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		server.HTTPError(w, http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	if err := server.EnsureKeys(q, "name"); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	j, tail, until, err := GetHttpSystemdJournal(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer j.Close()
	if tail > 0 {
		if err = j.SeekTail(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err = j.PreviousSkip(tail); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		if err = j.SeekHead(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	if _, err = j.GetCursor(); err != nil {
		http.Error(w, "404 Not Found", http.StatusNotFound)
		return
	}
	until_ts := uint64(until.UnixMicro())
	w.Header().Set("Content-Type", "application/json")
	sep := "["
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	for {
		e, err := j.GetEntry()
		if err != nil {
			panic(err)
		}
		if e.RealtimeTimestamp > until_ts {
			break
		}
		fmt.Fprint(w, sep)
		enc.Encode(&Message{
			TimeStamp: float64(e.RealtimeTimestamp) / 1000,
			Monotonic: float64(e.MonotonicTimestamp) / 1000000,
			HostName:  e.Fields[sdjournal.SD_JOURNAL_FIELD_HOSTNAME],
			Process:   e.Fields[sdjournal.SD_JOURNAL_FIELD_COMM],
			Pid:       e.Fields[sdjournal.SD_JOURNAL_FIELD_PID],
			Message:   e.Fields[sdjournal.SD_JOURNAL_FIELD_MESSAGE],
			Priority:  e.Fields[sdjournal.SD_JOURNAL_FIELD_PRIORITY],
		})
		sep = ","
		n, err := j.Next()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if n <= 0 {
			break
		}
	}
	fmt.Fprint(w, "]")
}

func (s *Server) HandleWatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		server.HTTPError(w, http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	if err := server.EnsureKeys(q, "name"); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	j, tail, until, err := GetHttpSystemdJournal(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer j.Close()

	if tail > 0 {
		if err = j.SeekTail(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err = j.PreviousSkip(tail); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		if err = j.SeekHead(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if _, err = j.GetCursor(); err != nil {
		fmt.Println("GetCursor error:", err)
		http.Error(w, "404 Not Found", http.StatusNotFound)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	until_ts := uint64(until.UnixMicro())
	hasNew := true

	for {
		select {
		case <-r.Context().Done():
			slog.Debug("监听停止", "reason", "客户端断开连接")
			return
		default:
			if hasNew {
				e, err := j.GetEntry()
				if err != nil {
					slog.Debug("监听停止", "reason", "获取Entry异常", "err", err)
					return
				}
				if e.RealtimeTimestamp > until_ts {
					slog.Debug("监听停止", "reason", "到达时间期限")
					return
				}
				fmt.Fprint(w, "data: ")
				if err = enc.Encode(&Message{
					TimeStamp: float64(e.RealtimeTimestamp) / 1000,
					Monotonic: float64(e.MonotonicTimestamp) / 1000000,
					HostName:  e.Fields[sdjournal.SD_JOURNAL_FIELD_HOSTNAME],
					Process:   e.Fields[sdjournal.SD_JOURNAL_FIELD_COMM],
					Pid:       e.Fields[sdjournal.SD_JOURNAL_FIELD_PID],
					Message:   e.Fields[sdjournal.SD_JOURNAL_FIELD_MESSAGE],
					Priority:  e.Fields[sdjournal.SD_JOURNAL_FIELD_PRIORITY],
				}); err != nil {
					slog.Debug("监听停止", "reason", "Json序列化异常", "err", err)
					return
				}
				fmt.Fprint(w, "\n")
				flusher.Flush()
			}
			if n, err := j.Next(); err != nil {
				slog.Debug("监听停止", "reason", "获取下一条Entry异常", "err", err)
				return
			} else if n <= 0 {
				status := j.Wait(200 * time.Microsecond)
				if time.Until(until) <= 0 {
					slog.Debug("监听停止", "reason", "到达时间期限")
					return
				}
				hasNew = status == sdjournal.SD_JOURNAL_APPEND
				continue
			}
		}
	}
}

func init() {
	server.Register("systemd", NewServer)
}
