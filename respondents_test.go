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
var columns = []string{"id", "email_address", "first_name", "last_name", "telephone", "status", "business_id"}

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

	returnRows := mock.NewRows(columns)
	returnRows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71", "bob@boblaw.com", "Bob", "Boblaw", "01234567890", "ACTIVE", "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2")
	returnRows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71", "bob@boblaw.com", "Bob", "Boblaw", "01234567890", "ACTIVE", "2711912c-db86-4e1e-9728-fc28db049858")

	mock.ExpectQuery(queryRegex).WillReturnRows(returnRows)
	req := httptest.NewRequest("GET",
		"/v2/respondents?firstName=Bob&lastName=Boblaw&emailAddress=bob@boblaw.com&telephone=01234567890&status=ACTIVE"+
			"&businessId=21ab28e5-28cc-4a53-8186-e19d6942002c&surveyId=0ee5265c-9cf3-4029-a07e-db1e1d94a499&offset=15&limit=10",
		nil)
	router.ServeHTTP(resp, req)
	body, _ := ioutil.ReadAll(resp.Body)

	var respondent models.Respondents
	err = json.Unmarshal(body, &respondent)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'GET /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, 1, len(respondent.Data))
	assert.Equal(t, "be70e086-7bbc-461c-a565-5b454d748a71", respondent.Data[0].Attributes.ID)
	assert.Equal(t, 2, len(respondent.Data[0].Associations))
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

func TestGetRespondentsReturns404WhenNoResults(t *testing.T) {
	setup()
	toggleFeature("party.api.get.respondents", true)

	var mock sqlmock.Sqlmock
	var err error

	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	mock.ExpectQuery(queryRegex).WillReturnRows(mock.NewRows(columns))

	req := httptest.NewRequest("GET", "/v2/respondents?firstName=Bob", nil)
	router.ServeHTTP(resp, req)
	body, _ := ioutil.ReadAll(resp.Body)

	var errResp models.Error
	err = json.Unmarshal(body, &errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'GET /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusNotFound, resp.Code)
	assert.Equal(t, "No respondents found", errResp.Error)
}
