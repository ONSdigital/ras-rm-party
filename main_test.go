package main

import (
	"database/sql/driver"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

var router *httprouter.Router
var resp *httptest.ResponseRecorder

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

	addRoutes(router)
}

func TestStartServer(t *testing.T) {
	setDefaults()
	router := httprouter.New()
	testWg.Add(1)
	srv := startServer(router, &testWg)
	assert.Equal(t, ":"+viper.GetString("port"), srv.Addr)
	srv.Close()
}
