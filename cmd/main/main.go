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
	"github.com/boringcat/just-a-log-viewer/web"
)

var (
	G_bufsize units.Base2Bytes
	listen    *net.TCPAddr
	cmdServer *kingpin.CmdClause
	globTest  *kingpin.CmdClause
)

func parserArgs() string {
	app := kingpin.New("log-viewer", "")
	debug := app.Flag("debug", "输出Debug日志").Short('d').Bool()

	cmdServer = app.Command("server", "启动服务").Default()
	cmdServer.Flag("config", "配置文件路径").Short('c').ExistingFileVar(&dirfiles.ConfigFile)
	cmdServer.Flag("listen", "监听地址").Default(":8514").Short('l').TCPVar(&listen)
	cmdServer.Flag("systemd", "启用Systemd日志功能").BoolVar(&journald.Enabled)
	cmdServer.Flag("systemd-unit-state", "获取systemd unit的state过滤").Default("running,exited,failed,dead").StringVar(&journald.SystemdUnitState)
	cmdServer.Flag("docker", "启用Docker日志功能").BoolVar(&docker.Enabled)
	cmdServer.Flag("buffer", "文件扫描缓冲区大小").Default("16KiB").BytesVar(&G_bufsize)

	tools := app.Command("tools", "工具")
	globTest = tools.Command("glob-test", "测试glob配置")
	globTest.Flag("config", "配置文件路径").Short('c').Required().ExistingFileVar(&dirfiles.ConfigFile)

	cmd := kingpin.MustParse(app.Parse(os.Args[1:]))
	server.GlobalBufSize = int(G_bufsize)

	if debug != nil && *debug {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}
	return cmd
}

func serverMain() {
	mux := server.NewHttpMux()
	mux.Handle("/", web.MustGetWebHandler())
	if err := http.ListenAndServe(listen.String(), mux); err != nil {
		panic(err)
	}
}

func main() {
	switch parserArgs() {
	case globTest.FullCommand():
		globTestMain()
	case cmdServer.FullCommand():
		serverMain()
	default:
		serverMain()
	}
}
