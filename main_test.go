package main

import (
	"database/sql/driver"
	"log"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Unleash/unleash-client-go/v3"
	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

var router *httprouter.Router
var resp *httptest.ResponseRecorder
var unleashStub *fakeUnleashServer

var testWg sync.WaitGroup

// Matching functions for sqlmock
type AnyUUID struct{}

func (a AnyUUID) Match(v driver.Value) bool {
	_, err := uuid.Parse(v.(string))
	return err == nil
}

type AnyTime struct{}

func (a AnyTime) Match(v driver.Value) bool {
	_, ok := v.(time.Time)
	return ok
}

func setup() {
	setDefaults()
	router = httprouter.New()
	resp = httptest.NewRecorder()
	if unleashStub == nil {
		unleashStub = newFakeUnleash()
		err := unleash.Initialize(unleash.WithUrl(unleashStub.url()),
			unleash.WithAppName("ras-rm-party test"),
			unleash.WithListener(unleash.DebugListener{}),
			unleash.WithRefreshInterval(time.Second*1))

		if err != nil {
			log.Fatal("Couldn't start an Unleash stub:", err)
		}
	}

	addRoutes(router)
}

func toggleFeature(feature string, on bool) {
	if unleashStub.IsEnabled(feature) != on {
		unleashStub.setEnabled(feature, on)
		// Required to let the unleash stub poll for new settings
		time.Sleep(time.Millisecond * 1500)
	}
}

func TestStartServer(t *testing.T) {
	setDefaults()
	router := httprouter.New()
	testWg.Add(1)
	srv := startServer(router, &testWg)
	assert.Equal(t, ":"+viper.GetString("port"), srv.Addr)
	srv.Close()
}
