package main

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/alecthomas/units"
	"github.com/boringcat/just-a-log-viewer/dirfiles"
	"github.com/boringcat/just-a-log-viewer/docker"
	"github.com/boringcat/just-a-log-viewer/journald"
	"github.com/boringcat/just-a-log-viewer/server"
	"github.com/boringcat/just-a-log-viewer/web"
)

var (
	G_bufsize      units.Base2Bytes
	listen         *net.TCPAddr
	prefix         string
	prefixRedirect bool
	cmdServer      *kingpin.CmdClause
	printVersion   *kingpin.CmdClause
	globTest       *kingpin.CmdClause
	compOpt        = server.CompressOpts{}

	version, buildDate, commit, goVersion, gitBranch string
)

func parserArgs() string {
	app := kingpin.New("log-viewer", "")
	debug := app.Flag("debug", "输出Debug日志").Short('d').Bool()

	cmdServer = app.Command("server", "启动服务").Default()
	cmdServer.Flag("config", "配置文件路径").Short('c').ExistingFileVar(&dirfiles.ConfigFilePath)
	cmdServer.Flag("listen", "监听地址").Default(":8514").Short('l').TCPVar(&listen)
	cmdServer.Flag("systemd", "启用Systemd日志功能").BoolVar(&journald.Enabled)
	cmdServer.Flag("systemd-unit-state", "获取systemd unit的state过滤").Default("running,exited,failed,dead").StringVar(&journald.SystemdUnitState)
	cmdServer.Flag("docker", "启用Docker日志功能").BoolVar(&docker.Enabled)
	cmdServer.Flag("docker-all-container", "列出所有docker容器").BoolVar(&docker.AllContainer)
	cmdServer.Flag("buffer", "文件扫描缓冲区大小").Default("16KiB").BytesVar(&G_bufsize)
	cmdServer.Flag("prefix", "HTTP服务前缀").StringVar(&prefix)
	cmdServer.Flag("prefix-redirect", "启用前缀跳转").BoolVar(&prefixRedirect)
	cmdServer.Flag("compress-order", "HTTP压缩顺序").Default(server.SupportedCompress...).EnumsVar(&compOpt.Order, server.SupportedCompress...)
	cmdServer.Flag("compress-gzip-level", "HTTP Gzip压缩等级").Default("-1").IntVar(&compOpt.GzipLevel)
	cmdServer.Flag("compress-deflate-level", "HTTP Deflate压缩等级").Default("-1").IntVar(&compOpt.DeflateLevel)
	cmdServer.Flag("compress-br-level", "HTTP Brotil压缩等级").Default("6").IntVar(&compOpt.BrotilLevel)
	cmdServer.Flag("compress-zstd-level", "HTTP Zstd压缩等级").Default("1").IntVar(&compOpt.ZstdLevel)

	tools := app.Command("tools", "工具")
	globTest = tools.Command("glob-test", "测试glob配置")
	globTest.Flag("config", "配置文件路径").Short('c').Required().ExistingFileVar(&dirfiles.ConfigFilePath)

	printVersion = app.Command("version", "打印版本号")

	cmd := kingpin.MustParse(app.Parse(os.Args[1:]))
	if cmd == cmdServer.FullCommand() {
		server.GlobalBufSize = int(G_bufsize)
		compOpt.Verify()
	}

	if debug != nil && *debug {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}
	return cmd
}

func serverMain() {
	if len(prefix) > 0 && !strings.HasPrefix(prefix, "/") {
		prefix = fmt.Sprintf("/%s", prefix)
	}
	mux := server.NewHttpMux(prefix)
	mux.Handle(fmt.Sprintf("%s/", prefix), web.MustGetWebHandler(prefix))
	if len(prefix) > 0 && prefixRedirect {
		mux.HandleFunc("/", server.RedirectPrefix(prefix))
		fmt.Println(prefixRedirect)
	}
	if err := http.ListenAndServe(listen.String(), server.NewCompressHandler(mux, &compOpt)); err != nil {
		panic(err)
	}
}

func printVersionMain() {
	fmt.Printf("just-a-log-viewer, version %s (branch: %s, revision: %s)\n", version, gitBranch, commit)
	fmt.Printf("  build date: %s\n", buildDate)
	fmt.Printf("  go version: %s\n", goVersion)
	fmt.Printf("  platform:   %s/%s\n", runtime.GOOS, runtime.GOARCH)
}

func main() {
	switch parserArgs() {
	case globTest.FullCommand():
		globTestMain()
	case cmdServer.FullCommand():
		serverMain()
	case printVersion.FullCommand():
		printVersionMain()
	default:
		serverMain()
	}
}
