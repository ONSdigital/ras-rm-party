package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetRespondents(t *testing.T) {
	setDefaults()
	setup()

	turnFeatureOn("party.api.get.respondents")

	req := httptest.NewRequest("GET", "/v2/respondents/", nil)
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusNotImplemented, resp.Code)
}
