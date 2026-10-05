package jsonhelp

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
)

type MapParser[RequestT any] func(data map[string]string) (RequestT, error)

func HandleWithMap[RequestT any, ResponseT any](w http.ResponseWriter, r *http.Request, parser MapParser[RequestT], handler HandleFunc[RequestT, ResponseT]) {
	vars := mux.Vars(r)
	q, err := parser(vars)
	if err != nil {
		http.Error(w, fmt.Sprintf("parsing request: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	handleAndWrite(w, r, q, handler)
}
