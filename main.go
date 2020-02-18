package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/ONSdigital/ras-rm-party/models"
	"github.com/Unleash/unleash-client-go/v3"
	"github.com/julienschmidt/httprouter"
	"github.com/spf13/viper"
)

var wg sync.WaitGroup

func hello(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if unleash.IsEnabled("party.api.get.hello", unleash.WithFallback(false)) {
		fmt.Fprint(w, viper.GetString("service_name"))
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func info(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	info := models.Info{
		Name:    viper.GetString("service_name"),
		Version: viper.GetString("app_version"),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

func addRoutes(r *httprouter.Router) {
	r.GET("/v2/", hello)
	r.GET("/v2/info/", info)
}

func startServer(r http.Handler) *http.Server {
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
	srv := startServer(router)
	wg.Wait()

	log.Println("Shutting down Party service...")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal(err)
	}
}
