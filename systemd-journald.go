package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"sort"
	"strconv"
	"time"

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

func HandleSystemdList(w http.ResponseWriter, r *http.Request) {
	units := Units{}
	p := exec.Command("/usr/bin/env", "systemctl", "list-units", "--state=running,exited,failed", "-o", "json")
	out, err := p.StdoutPipe()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer out.Close()
	if err := p.Start(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := json.NewDecoder(out).Decode(&units); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := p.Wait(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sort.Sort(units)
	w.Header().Set("Content-Type", "application/json")
	sep := "["
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	for _, unit := range units {
		fmt.Fprint(w, sep)
		enc.Encode(unit.Name)
		sep = ","
	}
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

func HandleSystemdTail(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if err := EnsureKeys(q, "name"); err != nil {
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

func HandleSystemdWatch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if err := EnsureKeys(q, "name"); err != nil {
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
			fmt.Println("client disconnected")
			return
		default:
			if hasNew {
				e, err := j.GetEntry()
				if err != nil {
                    fmt.Println("get entry error:", err)
                    return
				}
				if e.RealtimeTimestamp > until_ts {
                    fmt.Println("after until_ts")
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
					fmt.Println("enc error", err)
					return
				}
                fmt.Fprint(w, "\n")
				flusher.Flush()
			}
			if n, err := j.Next(); err != nil {
				fmt.Println("next error", err)
				return
			} else if n <= 0 {
				status := j.Wait(200 * time.Microsecond)
				if time.Now().After(until) {
                    fmt.Println("after until:", until)
					return
				}
				hasNew = status == sdjournal.SD_JOURNAL_APPEND
				continue
			}
		}
	}
}
