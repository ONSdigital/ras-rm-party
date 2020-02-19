package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ONSdigital/ras-rm-party/models"
	"github.com/stretchr/testify/assert"
)

var queryRegex = "SELECT (.+) from partysvc.respondent r JOIN partysvc.enrolment e ON r.id=e.respondent_id.*"

func TestGetRespondentsIsFeatureFlagged(t *testing.T) {
	// Assure that it's properly feature flagged away
	setDefaults()
	setup()
	toggleFeature("party.api.get.respondents", false)

	req := httptest.NewRequest("GET", "/v2/respondents", nil)
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusMethodNotAllowed, resp.Code)
}

func TestGetRespondents(t *testing.T) {
	setup()
	toggleFeature("party.api.get.respondents", true)

	var mock sqlmock.Sqlmock
	var err error

	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	fakeResult := []string{"id", "trading_as"}

	mock.ExpectQuery(queryRegex).
		WillReturnRows(mock.NewRows(fakeResult).AddRow(1, "Fake Co Inc"))
	req := httptest.NewRequest("GET",
		"/v2/respondents?firstName=Bob&lastName=Boblaw&emailAddress=bob@boblaw.com&telephone=01234567890&status=ACTIVE"+
			"&businessId=21ab28e5-28cc-4a53-8186-e19d6942002c&surveyId=0ee5265c-9cf3-4029-a07e-db1e1d94a499&offset=15&limit=10",
		nil)
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusNotImplemented, resp.Code)
}

func TestGetRespondentsReturns400WhenNoParamsProvided(t *testing.T) {
	setup()
	toggleFeature("party.api.get.respondents", true)

	req := httptest.NewRequest("GET", "/v2/respondents", nil)
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusBadRequest, resp.Code)
}

func TestGetRespondentsReturns400WhenBadParamProvided(t *testing.T) {
	setup()
	toggleFeature("party.api.get.respondents", true)

	req := httptest.NewRequest("GET", "/v2/respondents?nonsense=true", nil)
	router.ServeHTTP(resp, req)
	body, _ := ioutil.ReadAll(resp.Body)

	assert.Equal(t, http.StatusBadRequest, resp.Code)

	var errResp models.Error
	err := json.Unmarshal(body, &errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'GET /respondents', ", err.Error())
	}

	assert.Equal(t, "Invalid query parameter nonsense", errResp.Error)
}

func TestGetRespondentsReturns404WhenDBNotInit(t *testing.T) {
	// It shouldn't be possible to start the app without a DB, but just in case
	setup()
	toggleFeature("party.api.get.respondents", true)
	db = nil

	req := httptest.NewRequest("GET", "/v2/respondents?firstName=Bob", nil)
	router.ServeHTTP(resp, req)
	body, _ := ioutil.ReadAll(resp.Body)

	var errResp models.Error
	err := json.Unmarshal(body, &errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'GET /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusNotFound, resp.Code)
	assert.Equal(t, "Database connection could not be found", errResp.Error)
}

func TestGetRespondentsReturns404WhenDBDown(t *testing.T) {
	setup()
	toggleFeature("party.api.get.respondents", true)

	var mock sqlmock.Sqlmock
	var err error

	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	mock.ExpectQuery(queryRegex).WillReturnError(fmt.Errorf("Connection refused"))

	req := httptest.NewRequest("GET", "/v2/respondents?firstName=Bob", nil)
	router.ServeHTTP(resp, req)
	body, _ := ioutil.ReadAll(resp.Body)

	var errResp models.Error
	err = json.Unmarshal(body, &errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'GET /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusNotFound, resp.Code)
	assert.Equal(t, "Error querying DB: Connection refused", errResp.Error)
}
