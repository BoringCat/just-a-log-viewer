package main

import (
	"os"

	"github.com/alecthomas/kingpin/v2"
)

var (
	glob_test  *kingpin.CmdClause
	configFile string
)

func parserArgs() string {
	app := kingpin.New("tools", "工具")
	glob_test := app.Command("glob-test", "测试glob配置")
	glob_test.Flag("config", "配置文件路径").Short('c').Required().ExistingFileVar(&configFile)

	return kingpin.MustParse(app.Parse(os.Args[1:]))
}

func main() {
	switch parserArgs() {
	case glob_test.FullCommand():
		globTestMain()
	}
}
