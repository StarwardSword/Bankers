package jsonhelp

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func ParsePost[T any](r *http.Request) (T, error) {
	defer r.Body.Close()

	var res T
	err := json.NewDecoder(r.Body).Decode(&res)
	if err != nil {
		return res, fmt.Errorf("decoding json: %w", err)
	}

	return res, nil
}
