package main

import (
	"log/slog"

	"github.com/boringcat/just-a-log-viewer/dirfiles"
)

func globTestMain() {
	confs, err := dirfiles.ReadConfig(configFile)
	if err != nil {
		panic(err)
	}
	for f := range dirfiles.DoGlobWalk(confs) {
		slog.Info("找到文件", "path", f.Path, "key", f.Key, "name", f.Name)
	}
}
