package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

type Container struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func HandleDockerList(w http.ResponseWriter, r *http.Request) {
	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer apiClient.Close()

	containers, err := apiClient.ContainerList(r.Context(), types.ContainerListOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	sep := "["
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	for _, ctr := range containers {
		fmt.Fprint(w, sep)
		enc.Encode(Container{
			ID:   ctr.ID,
			Name: strings.TrimPrefix(ctr.Names[0], "/"),
		})
		sep = ","
	}
	fmt.Fprint(w, "]")
}

func HandleDockerTail(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if err := EnsureKeys(q, "id"); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer apiClient.Close()

	opts := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	}
	if q.Has("tail") {
		opts.Tail = q.Get("tail")
	}

	rd, err := apiClient.ContainerLogs(r.Context(), q.Get("id"), opts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rd.Close()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	scanner := bufio.NewScanner(rd)
	for scanner.Scan() {
		buf := scanner.Bytes()
		if len(buf) <= 8 {
			continue
		}
		w.Write(buf[8:])
		fmt.Fprintln(w)
	}
}

func HandleDockerWatch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if err := EnsureKeys(q, "id"); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.NotFound(w, r)
		return
	}

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer apiClient.Close()

	opts := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	}
	if q.Has("tail") {
		opts.Tail = q.Get("tail")
	}

	rd, err := apiClient.ContainerLogs(r.Context(), q.Get("id"), opts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rd.Close()

	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	scanner := bufio.NewScanner(rd)
	for scanner.Scan() {
		buf := scanner.Bytes()
		if len(buf) <= 8 {
			continue
		}
		fmt.Fprintf(w, "data: ")
		w.Write(buf[8:])
		fmt.Fprintln(w, "\n")
		flusher.Flush()
	}
}
