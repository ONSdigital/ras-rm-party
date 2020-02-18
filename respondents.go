package main

import (
	"log"
	"net/http"

	"github.com/Unleash/unleash-client-go/v3"
	"github.com/julienschmidt/httprouter"
)

func getRespondents(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if !unleash.IsEnabled("party.api.get.respondents", unleash.WithFallback(false)) {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if db == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	_, err := db.Query("SELECT * from respondents")
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		log.Println("Error querying DB:", err.Error())
		return
	}

	w.WriteHeader(http.StatusNotImplemented)
}
