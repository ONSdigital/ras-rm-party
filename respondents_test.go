package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestGetRespondents(t *testing.T) {
	setup()

	turnFeatureOn("party.api.get.respondents")

	var mock sqlmock.Sqlmock
	var err error

	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	fakeResult := []string{"id", "trading_as"}

	mock.ExpectQuery("SELECT (.+) from respondents").WillReturnRows(mock.NewRows(fakeResult).AddRow(1, "Fake Co Inc"))
	req := httptest.NewRequest("GET", "/v2/respondents/", nil)
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusNotImplemented, resp.Code)
}

func TestGetRespondentsIsFeatureFlagged(t *testing.T) {
	// Assure that it's properly feature flagged away
	setDefaults()
	setup()

	req := httptest.NewRequest("GET", "/v2/respondents/", nil)
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusMethodNotAllowed, resp.Code)
}

func TestGetRespondentsReturns404WhenDBNotInit(t *testing.T) {
	setup()
	turnFeatureOn("party.api.get.respondents")

	req := httptest.NewRequest("GET", "/v2/respondents/", nil)
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusNotFound, resp.Code)
}

func TestGetRespondentsReturns404WhenDBDown(t *testing.T) {
	setup()
	turnFeatureOn("party.api.get.respondents")

	var logOutput bytes.Buffer

	log.SetOutput(&logOutput)

	var mock sqlmock.Sqlmock
	var err error

	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	mock.ExpectQuery("SELECT (.+) from respondents").WillReturnError(fmt.Errorf("Connection refused"))

	req := httptest.NewRequest("GET", "/v2/respondents/", nil)
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusNotFound, resp.Code)
	assert.Contains(t, logOutput.String(), "Error querying DB: Connection refused")
}
