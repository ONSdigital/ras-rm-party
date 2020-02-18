package main

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/Unleash/unleash-client-go/v3"
	"github.com/julienschmidt/httprouter"
	"github.com/spf13/viper"
)

var wg sync.WaitGroup

func addRoutes(r *httprouter.Router) {
	r.GET("/v2/info/", getInfo)
	r.GET("/v2/respondents/", getRespondents)
}

func startServer(r http.Handler, wg *sync.WaitGroup) *http.Server {
	srv := &http.Server{
		Handler: r,
		Addr:    ":" + viper.GetString("port"),
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
	viper.AutomaticEnv()
	setDefaults()

	unleash.Initialize(unleash.WithListener(&unleash.DebugListener{}),
		unleash.WithAppName(viper.GetString("service_name")),
		unleash.WithUrl(viper.GetString("unleash_uri")))
	router := httprouter.New()
	addRoutes(router)

	wg.Add(1)
	srv := startServer(router, &wg)
	wg.Wait()

	log.Println("Shutting down Party service...")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal(err)
	}
}
