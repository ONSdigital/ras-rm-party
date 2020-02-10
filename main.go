package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/Unleash/unleash-client-go/v3"
	"github.com/julienschmidt/httprouter"
)

var wg sync.WaitGroup

func hello(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if unleash.IsEnabled("party.api.get.hello", unleash.WithFallback(true)) {
		fmt.Fprint(w, "ras-rm-party")
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func addRoutes(r *httprouter.Router) {
	r.GET("/", hello)
}

func startServer(r http.Handler) *http.Server {
	srv := &http.Server{
		Handler: r,
		Addr:    ":6969/v2/",
	}

	go func() {
		defer wg.Done()

		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatal("Panic running Party service API: ", err.Error())
		}
	}()

	return srv
}

func main() {
	unleash.Initialize(unleash.WithListener(&unleash.DebugListener{}),
		unleash.WithAppName("ras-rm-party"),
		unleash.WithUrl("http://localhost:4242/api"))
	router := httprouter.New()
	addRoutes(router)

	wg.Add(1)
	srv := startServer(router)
	wg.Wait()

	log.Println("Shutting down Party service...")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal(err)
	}
}
