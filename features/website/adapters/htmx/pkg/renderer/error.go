package renderer

import "net/http"

func errorComponent(w http.ResponseWriter, r *http.Request, err error) {
	comp := errorIndex(err.Error())
	comp.Render(r.Context(), w)
}
