package renderer

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/a-h/templ"
	"github.com/go-playground/form"
	"github.com/gorilla/mux"
)

var decoder *form.Decoder

func init() {
	decoder = form.NewDecoder()
}

type Handler[Service any, Query any] func(context.Context, Service, Query) (templ.Component, error)

func Render[Service any, Query any](w http.ResponseWriter, r *http.Request, service Service, handler Handler[Service, Query]) {
	var err error
	var value *Query

	if r.Method == "POST" || r.Method == "PUT" {
		value, err = postValue[Query](r)
	} else {
		value, err = getValue[Query](r)
	}

	if err != nil {
		errorComponent(w, r, fmt.Errorf("parsing query: %w", err))
		return
	}

	component, err := handler(r.Context(), service, *value)
	if err != nil {
		errorComponent(w, r, fmt.Errorf("runnig handler: %w", err))
		return
	}

	component.Render(r.Context(), w)
}

type CookieSetter func(w http.ResponseWriter) error

func RenderWithCookie[Service any, Query any](w http.ResponseWriter, r *http.Request, service Service, handler Handler[Service, Query], cookieSetter CookieSetter) {
	var err error
	var value *Query

	if r.Method == "POST" || r.Method == "PUT" {
		value, err = postValue[Query](r)
	} else {
		value, err = getValue[Query](r)
	}

	if err != nil {
		errorComponent(w, r, fmt.Errorf("parsing query: %w", err))
		return
	}

	component, err := handler(r.Context(), service, *value)
	if err != nil {
		errorComponent(w, r, fmt.Errorf("runnig handler: %w", err))
		return
	}

	err = cookieSetter(w)
	if err != nil {
		errorComponent(w, r, fmt.Errorf("setting cookie: %w", err))
		return
	}

	component.Render(r.Context(), w)
}

func getValue[Query any](r *http.Request) (*Query, error) {
	utlvals := url.Values{}
	vars := mux.Vars(r)

	for k, v := range vars {
		utlvals.Set(k, v)
	}

	value := new(Query)
	err := decoder.Decode(value, utlvals)
	if err != nil {
		return value, err
	}

	return value, err
}

func postValue[Query any](r *http.Request) (*Query, error) {
	defer r.Body.Close()
	r.ParseForm()

	value := new(Query)
	err := decoder.Decode(value, r.Form)
	if err != nil {
		return value, err
	}

	return value, err
}
