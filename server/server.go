package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
)

func NewHttpMux(prefix string) *http.ServeMux {
	mux := http.NewServeMux()
	futures.Range(func(key, value any) bool {
		slog.Debug("初始化模块", "future", key)
		obj, err := value.(NewServerFunc)()
		if err != nil {
			slog.Error("模块初始化失败", "future", key, "err", err)
			return true
		} else if obj == nil {
			slog.Info("跳过空模块初始化", "future", key)
			return true
		}
		mux.HandleFunc(fmt.Sprintf("%s/api/v%d/%s/list", prefix, API_VERSION, key), obj.HandleList)
		mux.HandleFunc(fmt.Sprintf("%s/api/v%d/%s/tail", prefix, API_VERSION, key), obj.HandleTail)
		mux.HandleFunc(fmt.Sprintf("%s/api/v%d/%s/watch", prefix, API_VERSION, key), obj.HandleWatch)
		enableFutures = append(enableFutures, key.(string))
		return true
	})
	futures_data, err := json.Marshal(enableFutures)
	if err != nil {
		panic(err)
	}
	mux.HandleFunc(fmt.Sprintf("%s/api/v%d/futures", prefix, API_VERSION), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(futures_data)
	})
	return mux
}
