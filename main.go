package main

import (
	"fmt"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func hello(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	fmt.Fprint(w, "ras-rm-party")
}

func addRoutes(r *httprouter.Router) {
	r.GET("/", hello)
}

func main() {
	router := httprouter.New()
	addRoutes(router)
}
