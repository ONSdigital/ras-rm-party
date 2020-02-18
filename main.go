package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/Unleash/unleash-client-go/v3"
	"github.com/julienschmidt/httprouter"
	"github.com/spf13/viper"

	_ "github.com/lib/pq"
)

var wg sync.WaitGroup
var db *sql.DB

func addRoutes(r *httprouter.Router) {
	r.GET("/v2/info", getInfo)
	r.GET("/v2/respondents", getRespondents)
}

func startServer(r http.Handler, wg *sync.WaitGroup) *http.Server {
	srv := &http.Server{
		Handler: r,
		Addr:    ":" + viper.GetString("port"),
	}

	go func() {
		defer wg.Done()

		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatal("Panic running Party service API:", err.Error())
		}
	}()

	return srv
}

func connectToDB() (*sql.DB, error) {
	db, err := sql.Open("postgres", viper.GetString("database_uri"))
	if err != nil {
		return nil, err
	}
	if err = db.Ping(); err != nil {
		return nil, err
	}
	return db, nil
}

func main() {
	// Config
	viper.AutomaticEnv()
	setDefaults()

	// Feature flagging
	unleash.Initialize(unleash.WithListener(&unleash.DebugListener{}),
		unleash.WithAppName(viper.GetString("service_name")),
		unleash.WithUrl(viper.GetString("unleash_uri")))

	var err error
	if db, err = connectToDB(); err != nil {
		log.Fatal("Error connecting to Postgres:", err.Error())
	}

	router := httprouter.New()
	addRoutes(router)

	wg.Add(1)
	srv := startServer(router, &wg)
	wg.Wait()

	log.Println("Shutting down Party service...")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	if err = srv.Shutdown(ctx); err != nil {
		log.Fatal(err)
	}
}
