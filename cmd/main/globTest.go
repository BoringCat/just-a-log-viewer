package main

import (
	"fmt"
	"log/slog"

	"github.com/boringcat/just-a-log-viewer/dirfiles"
)

func globTestMain() {
	confs, err := dirfiles.ReadConfig(dirfiles.ConfigFilePath)
	if err != nil {
		panic(err)
	}
	slog.Debug("加载到配置文件", "conf", confs)
	for f := range dirfiles.DoGlobWalk(confs) {
		args := make([]any, len(confs.Keys)*2+4)
		args[0], args[1], args[2], args[3] = "path", f.Path, "name", f.Name
		for idx, k := range confs.Keys {
			args[idx*2+4] = fmt.Sprintf("labels.%s", k)
			args[idx*2+4+1] = f.Labels[k]
		}
		slog.Info("找到文件", args...)
	}
}
