package jsonhelp

import (
	"context"
	"fmt"
	"net/http"
)

type HandleFunc[RequestT any, ResponseT any] func(ctx context.Context, r RequestT) (ResponseT, error)

func HandleWithBody[RequestT any, ResponseT any](w http.ResponseWriter, r *http.Request, handler HandleFunc[RequestT, ResponseT]) {
	q, err := ParsePost[RequestT](r)
	if err != nil {
		http.Error(w, fmt.Sprintf("parsing post request: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	handleAndWrite(w, r, q, handler)
}
