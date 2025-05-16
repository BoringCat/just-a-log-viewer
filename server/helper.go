package server

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const API_VERSION = 1

var (
	GlobalBufSize int = 16384
	enableFutures     = []string{}
)

func HTTPError(w http.ResponseWriter, code int) {
	http.Error(
		w,
		fmt.Sprintf("%d %s\n", code, http.StatusText(code)),
		code,
	)
}

func EnsureKeys(q url.Values, keys ...string) error {
	missing := strings.Builder{}
	sep := ""
	for _, key := range keys {
		if q.Has(key) {
			continue
		}
		fmt.Fprintf(&missing, "%q%s", key, sep)
		sep = ","
	}
	if len(sep) > 0 {
		return fmt.Errorf("missing query fields: [%s]", missing.String())
	}
	return nil
}
