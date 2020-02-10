package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Unleash/unleash-client-go/v3"

	"github.com/julienschmidt/httprouter"
)

var router *httprouter.Router
var resp *httptest.ResponseRecorder
var unleashStub *fakeUnleashServer

func setup() {
	router = httprouter.New()
	addRoutes(router)
	resp = httptest.NewRecorder()
	unleashStub = newFakeUnleash()
	err := unleash.Initialize(unleash.WithUrl(unleashStub.url()),
		unleash.WithAppName("ras-rm-party test"),
		unleash.WithListener(unleash.DebugListener{}))

	if err != nil {
		log.Fatal("Couldn't start an Unleash stub: ", err)
	}
}

func TestHello(t *testing.T) {
	setup()
	req := httptest.NewRequest("GET", "/v2/", nil)
	router.ServeHTTP(resp, req)
	body, _ := ioutil.ReadAll(resp.Body)

	if resp.Code != http.StatusOK {
		t.Fatal("Status code mismatch on 'GET /', expected ", http.StatusOK, " got ", resp.Code)
	}

	if string(body) != "ras-rm-party" {
		t.Fatal("Body mismatch on 'GET /', expected ras-rm-party got ", body)
	}
}
