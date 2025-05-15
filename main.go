package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/alecthomas/units"
)

var (
	G_bufsize       units.Base2Bytes
	configFile      string
	listen          *net.TCPAddr
	journalEnabled  bool
	dirfilesEnabled bool
	dockerEnabled   bool
)

func EnsureKeys(q url.Values, keys ...string) error {
	missing := strings.Builder{}
	sep := ""
	for _, key := range keys {
		if q.Has(key) {
			continue
		}
		fmt.Fprintf(&missing, "%q%s", key, sep)
		sep = ","
	}
	if len(sep) > 0 {
		return fmt.Errorf("missing query fields: [%s]", missing.String())
	}
	return nil
}

func parserArgs() {
	app := kingpin.New("log-viewer", "")
	app.Flag("config", "配置文件路径").Short('c').ExistingFileVar(&configFile)
	app.Flag("listen", "监听地址").Default(":8514").Short('l').TCPVar(&listen)
	app.Flag("systemd", "启用Systemd日志功能").BoolVar(&journalEnabled)
	app.Flag("docker", "启用Docker日志功能").BoolVar(&dockerEnabled)
	app.Flag("buffer", "文件扫描缓冲区大小").Default("16KiB").BytesVar(&G_bufsize)

	kingpin.MustParse(app.Parse(os.Args[1:]))
}

func HandleFutures(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	sep := "["
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if dirfilesEnabled {
		fmt.Fprint(w, sep)
		enc.Encode("dirfiles")
		sep = ","
	}
	if journalEnabled {
		fmt.Fprint(w, sep)
		enc.Encode("systemd")
		sep = ","
	}
	if dockerEnabled {
		fmt.Fprint(w, sep)
		enc.Encode("docker")
		sep = ","
	}
	fmt.Fprint(w, "]")
}

func main() {
	parserArgs()
	if len(configFile) > 0 {
		conf, err := ReadConfig(configFile)
		if err != nil {
			panic(err)
		}
		http.HandleFunc("/api/v1/dirfiles/services", conf.HandleList)
		http.HandleFunc("/api/v1/dirfiles/logs", conf.HandleTail)
		http.HandleFunc("/api/v1/dirfiles/stream", conf.HandleWatch)
		dirfilesEnabled = true
	}
	if journalEnabled {
		http.HandleFunc("/api/v1/systemd/services", HandleSystemdList)
		http.HandleFunc("/api/v1/systemd/logs", HandleSystemdTail)
		http.HandleFunc("/api/v1/systemd/stream", HandleSystemdWatch)
	}
	if dockerEnabled {
		http.HandleFunc("/api/v1/docker/services", HandleDockerList)
		http.HandleFunc("/api/v1/docker/logs", HandleDockerTail)
		http.HandleFunc("/api/v1/docker/stream", HandleDockerWatch)
	}
	http.HandleFunc("/api/v1/futures", HandleFutures)
	http.Handle("/", MustGetWebHandler())
	http.ListenAndServe(listen.String(), nil)
}
