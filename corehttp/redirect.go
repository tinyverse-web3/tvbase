package corehttp

import (
	"net"
	"net/http"

	tvCommon "github.com/tinyverse-web3/tvbase/common"
)

func RedirectOption(path string, redirect string) ServeOption {
	return func(t tvCommon.TvBaseService, _ net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {
		headers := map[string][]string{}
		handler := &redirectHandler{redirect, headers}

		if len(path) > 0 {
			mux.Handle("/"+path+"/", handler)
		} else {
			mux.Handle("/", handler)
		}
		return mux, nil
	}
}

type redirectHandler struct {
	path    string
	headers map[string][]string
}

func (i *redirectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for k, v := range i.headers {
		w.Header()[http.CanonicalHeaderKey(k)] = v
	}

	http.Redirect(w, r, i.path, http.StatusFound)
}
