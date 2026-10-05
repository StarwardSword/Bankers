package jsonhelp

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func handleAndWrite[RequestT any, ResponseT any](w http.ResponseWriter, r *http.Request, q RequestT, handler HandleFunc[RequestT, ResponseT]) {
	response, err := handler(r.Context(), q)
	if err != nil {
		http.Error(w, fmt.Sprintf("running handler: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	res, err := json.Marshal(response)
	if err != nil {
		http.Error(w, fmt.Sprintf("marshaling response: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	w.Write(res)
}
