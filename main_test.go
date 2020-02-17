package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ONSdigital/ras-rm-party/models"
	"github.com/Unleash/unleash-client-go/v3"
	"github.com/julienschmidt/httprouter"
)

var router *httprouter.Router
var resp *httptest.ResponseRecorder
var unleashStub *fakeUnleashServer

func setup() {
	setDefaults()
	router = httprouter.New()
	resp = httptest.NewRecorder()
	unleashStub = newFakeUnleash()
	err := unleash.Initialize(unleash.WithUrl(unleashStub.url()),
		unleash.WithAppName("ras-rm-party test"),
		unleash.WithListener(unleash.DebugListener{}),
		unleash.WithRefreshInterval(time.Second*1))

	if err != nil {
		log.Fatal("Couldn't start an Unleash stub: ", err)
	}

	addRoutes(router)
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
	// Required to let the unleash stub poll for new settings
	time.Sleep(time.Millisecond * 1500)

	req := httptest.NewRequest("GET", "/v2/info", nil)
	router.ServeHTTP(resp, req)
	body, _ := ioutil.ReadAll(resp.Body)

	if resp.Code != http.StatusOK {
		t.Fatal("Status code mismatch on 'GET /info', expected ", http.StatusOK, " got ", resp.Code)
	}

	var infoResp models.Info
	err := json.Unmarshal(body, &infoResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'GET /info', ", err.Error())
	}

	if infoResp.Name != "ras-rm-party" {
		t.Fatal("Name field received from 'GET /info' incorrect, expected ras-rm-party got ", infoResp.Name)
	}
}
