//go:build linux

package docker

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/boringcat/just-a-log-viewer/server"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

type Container struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Server struct {
	client *client.Client
}

func NewServer() (server.LogServer, error) {
	if Enabled {
		return &Server{}, nil
	}
	return nil, nil
}

func (s *Server) getClient(ctx context.Context) (*client.Client, error) {
	if s.client == nil {
		apiClient, err := client.NewClientWithOpts(client.FromEnv)
		if err != nil {
			return nil, err
		}
		s.client = apiClient
	} else {
		if _, err := s.client.Ping(ctx); err != nil {
			s.client.Close()
			s.client = nil
			if _, err = s.getClient(ctx); err != nil {
				return nil, err
			}
		}
	}
	return s.client, nil
}

func (s *Server) HandleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		server.HTTPError(w, http.StatusMethodNotAllowed)
		return
	}
	client, err := s.getClient(r.Context())
	containers, err := client.ContainerList(r.Context(), types.ContainerListOptions{All: AllContainer})
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

func (s *Server) HandleTail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		server.HTTPError(w, http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	if err := server.EnsureKeys(q, "id"); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	client, err := s.getClient(r.Context())
	opts := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	}
	if q.Has("tail") {
		opts.Tail = q.Get("tail")
	}

	rd, err := client.ContainerLogs(r.Context(), q.Get("id"), opts)
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

func (s *Server) HandleWatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		server.HTTPError(w, http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	if err := server.EnsureKeys(q, "id"); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.NotFound(w, r)
		return
	}

	client, err := s.getClient(r.Context())
	opts := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	}
	if q.Has("tail") {
		opts.Tail = q.Get("tail")
	}

	rd, err := client.ContainerLogs(r.Context(), q.Get("id"), opts)
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

func init() {
	server.Register("docker", NewServer)
}
