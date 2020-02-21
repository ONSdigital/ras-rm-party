package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ONSdigital/ras-rm-party/models"
	"github.com/stretchr/testify/assert"
)

var searchQueryRegex = "SELECT (.+) FROM partysvc.respondent JOIN partysvc.business_respondent br ON r.id=br.respondent_id JOIN partysvc.enrolment e ON br.business_id=e.business_id AND br.respondent_id=e.respondent_id*"
var searchQueryColumns = []string{"id", "email_address", "first_name", "last_name", "telephone", "status", "business_id", "enrolment_status", "survey_id"}
var insertQueryRegex = "INSERT INTO (.+)*"

// GET /respondents?...
func TestGetRespondentsIsFeatureFlagged(t *testing.T) {
	// Assure that it's properly feature flagged away
	setDefaults()
	setup()
	toggleFeature("party.api.get.respondents", false)

	req := httptest.NewRequest("GET", "/v2/respondents", nil)
	req.SetBasicAuth("admin", "secret")
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

	returnRows := mock.NewRows(searchQueryColumns)
	returnRows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71", "bob@boblaw.com", "Bob", "Boblaw", "01234567890", "ACTIVE", "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2", "ENABLED", "5e237abd-f8dc-4cb0-829e-58d5cef8ca4a")
	returnRows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71", "bob@boblaw.com", "Bob", "Boblaw", "01234567890", "ACTIVE", "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2", "DISABLED", "84bc0d0a-ae32-4fb1-aabc-6de370245d62")
	returnRows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71", "bob@boblaw.com", "Bob", "Boblaw", "01234567890", "ACTIVE", "2711912c-db86-4e1e-9728-fc28db049858", "ENABLED", "ba4274ac-a664-4c3d-8910-18b82a12ce09")
	returnRows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71", "bob@boblaw.com", "Bob", "Boblaw", "01234567890", "ACTIVE", "d4a6c190-50da-4d02-9a78-f4de52d9e6af", "", "")

	mock.ExpectQuery(searchQueryRegex).WillReturnRows(returnRows)
	req := httptest.NewRequest("GET",
		"/v2/respondents?firstName=Bob&lastName=Boblaw&emailAddress=bob@boblaw.com&telephone=01234567890&status=ACTIVE"+
			"&businessId=21ab28e5-28cc-4a53-8186-e19d6942002c&surveyId=0ee5265c-9cf3-4029-a07e-db1e1d94a499&offset=15&limit=10",
		nil)
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var respondent models.Respondents
	err = json.NewDecoder(resp.Body).Decode(&respondent)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'GET /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, 1, len(respondent.Data))
	assert.Equal(t, "be70e086-7bbc-461c-a565-5b454d748a71", respondent.Data[0].Attributes.ID)
	assert.Equal(t, 3, len(respondent.Data[0].Associations))
	assert.Equal(t, 2, len(respondent.Data[0].Associations[0].Enrolments))
	assert.Equal(t, 1, len(respondent.Data[0].Associations[1].Enrolments))
	assert.Equal(t, 0, len(respondent.Data[0].Associations[2].Enrolments))
}

func TestGetRespondentsReturns400WhenNoParamsProvided(t *testing.T) {
	setup()
	toggleFeature("party.api.get.respondents", true)

	req := httptest.NewRequest("GET", "/v2/respondents", nil)
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusBadRequest, resp.Code)
}

func TestGetRespondentsReturns400WhenBadParamProvided(t *testing.T) {
	setup()
	toggleFeature("party.api.get.respondents", true)

	req := httptest.NewRequest("GET", "/v2/respondents?nonsense=true", nil)
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusBadRequest, resp.Code)

	var errResp models.Error
	err := json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'GET /respondents', ", err.Error())
	}

	assert.Equal(t, "Invalid query parameter nonsense", errResp.Error)
}

func TestGetRespondentsReturns401WhenNotAuthed(t *testing.T) {
	setup()
	toggleFeature("party.api.get.respondents", true)

	req := httptest.NewRequest("GET",
		"/v2/respondents?firstName=Bob&lastName=Boblaw&emailAddress=bob@boblaw.com&telephone=01234567890&status=ACTIVE"+
			"&businessId=21ab28e5-28cc-4a53-8186-e19d6942002c&surveyId=0ee5265c-9cf3-4029-a07e-db1e1d94a499&offset=15&limit=10",
		nil)
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestGetRespondentsReturns404WhenDBNotInit(t *testing.T) {
	// It shouldn't be possible to start the app without a DB, but just in case
	setup()
	toggleFeature("party.api.get.respondents", true)
	db = nil

	req := httptest.NewRequest("GET", "/v2/respondents?firstName=Bob", nil)
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err := json.NewDecoder(resp.Body).Decode(&errResp)
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

	mock.ExpectQuery(searchQueryRegex).WillReturnError(fmt.Errorf("Connection refused"))

	req := httptest.NewRequest("GET", "/v2/respondents?firstName=Bob", nil)
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
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

	mock.ExpectQuery(searchQueryRegex).WillReturnRows(mock.NewRows(searchQueryColumns))

	req := httptest.NewRequest("GET", "/v2/respondents?firstName=Bob", nil)
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'GET /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusNotFound, resp.Code)
	assert.Equal(t, "No respondents found", errResp.Error)
}

// POST /respondents
func TestPostRespondentsIsFeatureFlagged(t *testing.T) {
	// Assure that it's properly feature flagged away
	setDefaults()
	setup()
	toggleFeature("party.api.post.respondents", false)

	req := httptest.NewRequest("POST", "/v2/respondents", nil)
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusMethodNotAllowed, resp.Code)
}

func TestPostRespondents(t *testing.T) {
	setup()
	toggleFeature("party.api.post.respondents", true)
	var mock sqlmock.Sqlmock
	var err error

	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	postReq := models.PostRespondents{
		Data: models.Respondent{
			Attributes: models.Attributes{
				EmailAddress: "bob@boblaw.com",
				FirstName:    "Bob",
				LastName:     "Boblaw",
				Telephone:    "01234567890",
			},
			Status: "ACTIVE",
		},
		EnrolmentCodes: []string{"abc1234"}}

	jsonOut, err := json.Marshal(postReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'POST /respondents', ", err.Error())
	}

	mock.ExpectBegin()
	mock.ExpectExec(insertQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	req := httptest.NewRequest("POST", "/v2/respondents", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusCreated, resp.Code)
}

func TestPostRespondentsReturns400IfBadJSON(t *testing.T) {
	setup()
	toggleFeature("party.api.post.respondents", true)

	var err error
	db, _, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	req := httptest.NewRequest("POST", "/v2/respondents", strings.NewReader("{nonsense: true}"))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'POST /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Equal(t, "Invalid JSON", errResp.Error)
}

func TestPostRespondentsReturns400IfRequiredFieldsMissing(t *testing.T) {
	setup()
	toggleFeature("party.api.post.respondents", true)

	var err error
	db, _, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	postReq := models.PostRespondents{
		Data: models.Respondent{
			Status: "ACTIVE",
		}}

	jsonOut, err := json.Marshal(postReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'POST /respondents', ", err.Error())
	}

	req := httptest.NewRequest("POST", "/v2/respondents", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'POST /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Equal(t, "Missing required fields: emailAddress, firstName, lastName, telephone, enrolmentCodes", errResp.Error)
}

func TestPostRespondentsReturns401WhenNotAuthed(t *testing.T) {
	setup()
	toggleFeature("party.api.post.respondents", true)
	postReq := models.PostRespondents{
		Data: models.Respondent{
			Attributes: models.Attributes{
				EmailAddress: "bob@boblaw.com",
				FirstName:    "Bob",
				LastName:     "Boblaw",
				Telephone:    "01234567890",
			},
			Status: "ACTIVE",
		},
		EnrolmentCodes: []string{"abc1234"}}

	jsonOut, err := json.Marshal(postReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'POST /respondents', ", err.Error())
	}

	req := httptest.NewRequest("POST", "/v2/respondents", bytes.NewBuffer(jsonOut))
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestPostRespondentsReturns404WhenDBNotInit(t *testing.T) {
	// It shouldn't be possible to start the app without a DB, but just in case
	setup()
	toggleFeature("party.api.post.respondents", true)
	db = nil

	postReq := models.PostRespondents{
		Data: models.Respondent{
			Attributes: models.Attributes{
				EmailAddress: "bob@boblaw.com",
				FirstName:    "Bob",
				LastName:     "Boblaw",
				Telephone:    "01234567890",
			},
			Status: "ACTIVE",
		},
		EnrolmentCodes: []string{"abc1234"}}

	jsonOut, err := json.Marshal(postReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'POST /respondents', ", err.Error())
	}

	req := httptest.NewRequest("POST", "/v2/respondents", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'GET /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusNotFound, resp.Code)
	assert.Equal(t, "Database connection could not be found", errResp.Error)
}

func TestPostRespondentsReturns404WhenDBDown(t *testing.T) {
	setup()
	toggleFeature("party.api.post.respondents", true)
	var mock sqlmock.Sqlmock
	var err error

	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	postReq := models.PostRespondents{
		Data: models.Respondent{
			Attributes: models.Attributes{
				EmailAddress: "bob@boblaw.com",
				FirstName:    "Bob",
				LastName:     "Boblaw",
				Telephone:    "01234567890",
			},
			Status: "ACTIVE",
		},
		EnrolmentCodes: []string{"abc1234"}}

	jsonOut, err := json.Marshal(postReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'POST /respondents', ", err.Error())
	}

	mock.ExpectBegin()
	mock.ExpectExec(insertQueryRegex).WillReturnError(fmt.Errorf("Connection refused"))
	mock.ExpectRollback()

	req := httptest.NewRequest("POST", "/v2/respondents", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'GET /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusNotFound, resp.Code)
	assert.Equal(t, "Error querying DB: Connection refused", errResp.Error)
}
