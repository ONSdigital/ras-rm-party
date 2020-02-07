package main

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/julienschmidt/httprouter"
)

var router *httprouter.Router
var resp *httptest.ResponseRecorder

func setup() {
	router = httprouter.New()
	addRoutes(router)
	resp = httptest.NewRecorder()
}

func TestHello(t *testing.T) {
	setup()
	req := httptest.NewRequest("GET", "/", nil)
	router.ServeHTTP(resp, req)
	body, _ := ioutil.ReadAll(resp.Body)

	if resp.Code != http.StatusOK {
		t.Fatal("Status code mismatch on 'GET /', expected ", http.StatusOK, " got ", resp.Code)
	}

	if string(body) != "ras-rm-party" {
		t.Fatal("Body mismatch on 'GET /', expected ras-rm-party got ", body)
	}
}
