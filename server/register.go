package server

import (
	"net/http"
	"sync"

	"github.com/pkg/errors"
)

var (
	futures                  sync.Map
	FutureAlreadyExistsError = errors.New("future already exists")
)

type LogServer interface {
	HandleList(w http.ResponseWriter, r *http.Request)
	HandleTail(w http.ResponseWriter, r *http.Request)
	HandleWatch(w http.ResponseWriter, r *http.Request)
}

type NewServerFunc func() (LogServer, error)

func Register(future string, fn NewServerFunc) {
	_, loaded := futures.LoadOrStore(future, fn)
	if loaded {
		panic(errors.Wrap(FutureAlreadyExistsError, future))
	}
}
