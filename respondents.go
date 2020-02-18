package main

import (
	"net/http"

	"github.com/Unleash/unleash-client-go/v3"
	"github.com/julienschmidt/httprouter"
)

func getRespondents(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if unleash.IsEnabled("party.api.get.respondents", unleash.WithFallback(false)) {
		w.WriteHeader(http.StatusNotImplemented)
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
