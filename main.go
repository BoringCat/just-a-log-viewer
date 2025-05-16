package main

import (
	"log/slog"
	"net"
	"net/http"
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/alecthomas/units"
	"github.com/boringcat/just-a-log-viewer/dirfiles"
	"github.com/boringcat/just-a-log-viewer/docker"
	"github.com/boringcat/just-a-log-viewer/journald"
	"github.com/boringcat/just-a-log-viewer/server"
)

var (
	G_bufsize units.Base2Bytes
	listen    *net.TCPAddr
)

func parserArgs() {
	app := kingpin.New("log-viewer", "")
	app.Flag("config", "配置文件路径").Short('c').ExistingFileVar(&dirfiles.ConfigFile)
	app.Flag("listen", "监听地址").Default(":8514").Short('l').TCPVar(&listen)
	app.Flag("systemd", "启用Systemd日志功能").BoolVar(&journald.Enabled)
	app.Flag("systemd-unit-state", "获取systemd unit的state过滤").Default("running,exited,failed,dead").StringVar(&journald.SystemdUnitState)
	app.Flag("docker", "启用Docker日志功能").BoolVar(&docker.Enabled)
	app.Flag("buffer", "文件扫描缓冲区大小").Default("16KiB").BytesVar(&G_bufsize)
	debug := app.Flag("debug", "输出Debug日志").Short('d').Bool()

	kingpin.MustParse(app.Parse(os.Args[1:]))
	server.GlobalBufSize = int(G_bufsize)

	if debug != nil && *debug {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}
}

func main() {
	parserArgs()

	mux := server.NewHttpMux()
	mux.Handle("/", MustGetWebHandler())
	http.ListenAndServe(listen.String(), mux)
}
