package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/Unleash/unleash-client-go/v3"
	"github.com/julienschmidt/httprouter"
)

func hello(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if unleash.IsEnabled("party.api.get.hello") {
		fmt.Fprint(w, "ras-rm-party")
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func addRoutes(r *httprouter.Router) {
	r.GET("/", hello)
}

func main() {
	unleash.Initialize(unleash.WithListener(&unleash.DebugListener{}),
		unleash.WithAppName("ras-rm-party"), unleash.WithUrl("http://localhost:4242/api"))
	router := httprouter.New()
	addRoutes(router)

	log.Fatal(http.ListenAndServe(":6969", router))
}
