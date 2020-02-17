package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Unleash/unleash-client-go/v3"

	"github.com/julienschmidt/httprouter"
)

type info struct {
	Name    string
	Version string
}

var router *httprouter.Router
var resp *httptest.ResponseRecorder
var unleashStub *fakeUnleashServer

func setup() {
	setDefaults()
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

func TestInfo(t *testing.T) {
	setup()
	unleashStub.Enable("party.api.get.info")

	req := httptest.NewRequest("GET", "/v2/info", nil)
	router.ServeHTTP(resp, req)
	body, _ := ioutil.ReadAll(resp.Body)

	if resp.Code != http.StatusOK {
		t.Fatal("Status code mismatch on 'GET /info', expected ", http.StatusOK, " got ", resp.Code)
	}

	var info info
	err := json.Unmarshal(body, &info)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'GET /info', ", err.Error())
	}

	if info.Name != "ras-rm-party" {
		t.Fatal("Name field received from 'GET /info' incorrect, expected ras-rm-party got ", info.Name)
	}
}
