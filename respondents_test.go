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
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"gopkg.in/h2non/gock.v1"
)

var searchQueryRegex = "SELECT (.+) FROM partysvc.respondent JOIN partysvc.business_respondent br ON r.id=br.respondent_id JOIN partysvc.enrolment e ON br.business_id=e.business_id AND br.respondent_id=e.respondent_id*"
var searchQueryColumns = []string{"id", "email_address", "first_name", "last_name", "telephone", "status", "business_id", "enrolment_status", "survey_id"}
var insertQueryRegex = "INSERT INTO (.+)*"
var copyQueryRegex = "COPY (.+) FROM STDIN"
var searchBusinessesQueryRegex = "SELECT (.+) FROM partysvc.business WHERE party_uuid=*"
var searchBusinessesQueryColumns = []string{"party_uuid"}

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
	// This wouldn't return all of the rows above IRL, but does test all our parameter logic
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
func TestGetRespondentsReturns500WhenDBNotInit(t *testing.T) {
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

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, "Database connection could not be found", errResp.Error)
}

func TestGetRespondentsReturns500WhenDBDown(t *testing.T) {
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

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, "Error querying DB: Connection refused", errResp.Error)
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
	defer gock.Off()
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

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(200).JSON(models.IAC{
		IAC:         "abc1234",
		Active:      true,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		QuestionSet: "H1"})

	gock.New("http://localhost:8171").Get("/cases/7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb").Reply(200).JSON(models.Case{
		ID:         "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		BusinessID: "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		CaseGroup: models.CaseGroup{
			ID:                   "aa9c8e93-5cd9-4876-a2d3-78a87b972134",
			CollectionExerciseID: "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		},
	})

	gock.New("http://localhost:8145").Get("/collectionexercises/1010b2f2-8668-498a-afee-3c33cdfe42ea").Reply(200).JSON(models.CollectionExercise{
		ID:       "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		SurveyID: "0752a892-1a60-40a4-8aa3-2599405a8831",
	})

	gock.New("http://localhost:8121").Put("/abc1234").Reply(200)

	jsonOut, err := json.Marshal(postReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'POST /respondents', ", err.Error())
	}

	businessRows := mock.NewRows(searchBusinessesQueryColumns)
	businessRows.AddRow("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2")

	mock.ExpectPrepare(searchBusinessesQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)

	mock.ExpectBegin()
	mock.ExpectPrepare(insertQueryRegex).ExpectExec().WithArgs(AnyUUID{}, "CREATED", postReq.Data.Attributes.EmailAddress, postReq.Data.Attributes.FirstName,
		postReq.Data.Attributes.LastName, postReq.Data.Attributes.Telephone, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectPrepare(copyQueryRegex).ExpectExec().WithArgs("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2", AnyUUID{}, "ACTIVE", AnyTime{}, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectPrepare(copyQueryRegex)
	mock.ExpectPrepare(copyQueryRegex)
	mock.ExpectExec(copyQueryRegex).WithArgs("7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb", AnyUUID{}, "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		"0752a892-1a60-40a4-8aa3-2599405a8831", AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WithArgs(AnyUUID{}, "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		"0752a892-1a60-40a4-8aa3-2599405a8831", "PENDING", AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectClose()

	req := httptest.NewRequest("POST", "/v2/respondents", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusCreated, resp.Code)
	assert.True(t, gock.IsDone())
}

func TestPostRespondentsIfIACDeactivationFails(t *testing.T) {
	// By not setting up the mock properly, we can effectively test an err in http PUT
	setup()
	toggleFeature("party.api.post.respondents", true)
	defer gock.Off()
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

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(200).JSON(models.IAC{
		IAC:         "abc1234",
		Active:      true,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		QuestionSet: "H1"})

	gock.New("http://localhost:8171").Get("/cases/7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb").Reply(200).JSON(models.Case{
		ID:         "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		BusinessID: "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		CaseGroup: models.CaseGroup{
			ID:                   "aa9c8e93-5cd9-4876-a2d3-78a87b972134",
			CollectionExerciseID: "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		},
	})

	gock.New("http://localhost:8145").Get("/collectionexercises/1010b2f2-8668-498a-afee-3c33cdfe42ea").Reply(200).JSON(models.CollectionExercise{
		ID:       "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		SurveyID: "0752a892-1a60-40a4-8aa3-2599405a8831",
	})

	// Calling IAC to deactivate the enrolment code fails, but the whole process still works and sends 200

	jsonOut, err := json.Marshal(postReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'POST /respondents', ", err.Error())
	}

	businessRows := mock.NewRows(searchBusinessesQueryColumns)
	businessRows.AddRow("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2")

	mock.ExpectPrepare(searchBusinessesQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)

	mock.ExpectBegin()
	mock.ExpectPrepare(insertQueryRegex).ExpectExec().WithArgs(AnyUUID{}, "CREATED", postReq.Data.Attributes.EmailAddress, postReq.Data.Attributes.FirstName,
		postReq.Data.Attributes.LastName, postReq.Data.Attributes.Telephone, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectPrepare(copyQueryRegex).ExpectExec().WithArgs("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2", AnyUUID{}, "ACTIVE", AnyTime{}, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectPrepare(copyQueryRegex)
	mock.ExpectPrepare(copyQueryRegex)
	mock.ExpectExec(copyQueryRegex).WithArgs("7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb", AnyUUID{}, "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		"0752a892-1a60-40a4-8aa3-2599405a8831", AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WithArgs(AnyUUID{}, "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		"0752a892-1a60-40a4-8aa3-2599405a8831", "PENDING", AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectClose()

	var logCatcher bytes.Buffer
	log.SetOutput(&logCatcher)

	req := httptest.NewRequest("POST", "/v2/respondents", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusCreated, resp.Code)
	assert.Contains(t, logCatcher.String(), "Error deactivating enrolment code abc1234:")
	assert.True(t, gock.IsDone())
}

func TestPostRespondentsIfIACDeactivationDoesntReturn200(t *testing.T) {
	setup()
	toggleFeature("party.api.post.respondents", true)
	defer gock.Off()
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

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(200).JSON(models.IAC{
		IAC:         "abc1234",
		Active:      true,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		QuestionSet: "H1"})

	gock.New("http://localhost:8171").Get("/cases/7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb").Reply(200).JSON(models.Case{
		ID:         "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		BusinessID: "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		CaseGroup: models.CaseGroup{
			ID:                   "aa9c8e93-5cd9-4876-a2d3-78a87b972134",
			CollectionExerciseID: "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		},
	})

	gock.New("http://localhost:8145").Get("/collectionexercises/1010b2f2-8668-498a-afee-3c33cdfe42ea").Reply(200).JSON(models.CollectionExercise{
		ID:       "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		SurveyID: "0752a892-1a60-40a4-8aa3-2599405a8831",
	})

	// Calling IAC to deactivate the enrolment code fails, but the whole process still works and sends 200
	gock.New("http://localhost:8121").Put("/abc1234").Reply(404)

	jsonOut, err := json.Marshal(postReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'POST /respondents', ", err.Error())
	}

	businessRows := mock.NewRows(searchBusinessesQueryColumns)
	businessRows.AddRow("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2")

	mock.ExpectPrepare(searchBusinessesQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)

	mock.ExpectBegin()
	mock.ExpectPrepare(insertQueryRegex).ExpectExec().WithArgs(AnyUUID{}, "CREATED", postReq.Data.Attributes.EmailAddress, postReq.Data.Attributes.FirstName,
		postReq.Data.Attributes.LastName, postReq.Data.Attributes.Telephone, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectPrepare(copyQueryRegex).ExpectExec().WithArgs("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2", AnyUUID{}, "ACTIVE", AnyTime{}, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectPrepare(copyQueryRegex)
	mock.ExpectPrepare(copyQueryRegex)
	mock.ExpectExec(copyQueryRegex).WithArgs("7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb", AnyUUID{}, "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		"0752a892-1a60-40a4-8aa3-2599405a8831", AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WithArgs(AnyUUID{}, "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		"0752a892-1a60-40a4-8aa3-2599405a8831", "PENDING", AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectClose()

	var logCatcher bytes.Buffer
	log.SetOutput(&logCatcher)

	req := httptest.NewRequest("POST", "/v2/respondents", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusCreated, resp.Code)
	assert.Contains(t, logCatcher.String(), "Error deactivating enrolment code abc1234: Received status code 404 from IAC service")
	assert.True(t, gock.IsDone())
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

func TestPostRespondentsReturns404IfEnrolmentCodeNotFound(t *testing.T) {
	setup()
	toggleFeature("party.api.post.respondents", true)
	defer gock.Off()
	var err error

	db, _, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(404)

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
	assert.Equal(t, "Enrolment code not found: abc1234", errResp.Error)
	assert.True(t, gock.IsDone())
}

func TestPostRespondentsReturns404IfCaseNotFound(t *testing.T) {
	setup()
	toggleFeature("party.api.post.respondents", true)
	defer gock.Off()
	var err error

	db, _, err = sqlmock.New()
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

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(200).JSON(models.IAC{
		IAC:         "abc1234",
		Active:      true,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		QuestionSet: "H1"})

	gock.New("http://localhost:8171").Get("/cases/7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb").Reply(404)

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
	assert.Equal(t, "Case not found for enrolment code: abc1234", errResp.Error)
	assert.True(t, gock.IsDone())
}

func TestPostRespondentsReturns404IfCollectionExerciseNotFound(t *testing.T) {
	setup()
	toggleFeature("party.api.post.respondents", true)
	defer gock.Off()
	var err error

	db, _, err = sqlmock.New()
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

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(200).JSON(models.IAC{
		IAC:         "abc1234",
		Active:      true,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		QuestionSet: "H1"})

	gock.New("http://localhost:8171").Get("/cases/7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb").Reply(200).JSON(models.Case{
		ID:         "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		BusinessID: "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		CaseGroup: models.CaseGroup{
			ID:                   "aa9c8e93-5cd9-4876-a2d3-78a87b972134",
			CollectionExerciseID: "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		},
	})

	gock.New("http://localhost:8145").Get("/collectionexercises/1010b2f2-8668-498a-afee-3c33cdfe42ea").Reply(404)

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
	assert.Equal(t, "Collection Exercise not found for enrolment code: abc1234", errResp.Error)
	assert.True(t, gock.IsDone())
}

func TestPostRespondentsReturns422IfEnrolmentCodeInactive(t *testing.T) {
	setup()
	toggleFeature("party.api.post.respondents", true)
	defer gock.Off()
	var err error

	db, _, err = sqlmock.New()
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

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(200).JSON(models.IAC{
		IAC:         "abc1234",
		Active:      false,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		QuestionSet: "H1"})

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

	assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	assert.Equal(t, "Enrolment code inactive: abc1234", errResp.Error)
	assert.True(t, gock.IsDone())
}

func TestPostRespondentsReturns422IfBusinessNotFoundToAssociate(t *testing.T) {
	setup()
	toggleFeature("party.api.post.respondents", true)
	defer gock.Off()
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

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(200).JSON(models.IAC{
		IAC:         "abc1234",
		Active:      true,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		QuestionSet: "H1"})

	gock.New("http://localhost:8171").Get("/cases/7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb").Reply(200).JSON(models.Case{
		ID:         "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		BusinessID: "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		CaseGroup: models.CaseGroup{
			ID:                   "aa9c8e93-5cd9-4876-a2d3-78a87b972134",
			CollectionExerciseID: "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		},
	})

	gock.New("http://localhost:8145").Get("/collectionexercises/1010b2f2-8668-498a-afee-3c33cdfe42ea").Reply(200).JSON(models.CollectionExercise{
		ID:       "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		SurveyID: "0752a892-1a60-40a4-8aa3-2599405a8831",
	})

	jsonOut, err := json.Marshal(postReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'POST /respondents', ", err.Error())
	}

	mock.ExpectPrepare(searchBusinessesQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(mock.NewRows(searchBusinessesQueryColumns))

	req := httptest.NewRequest("POST", "/v2/respondents", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'GET /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	assert.Equal(t, "Can't associate with the business for enrolment code: abc1234", errResp.Error)
	assert.True(t, gock.IsDone())
}

func TestPostRespondentsReturns422IfRespondentCouldntBeInserted(t *testing.T) {
	setup()
	toggleFeature("party.api.post.respondents", true)
	defer gock.Off()
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
				ID:           "be70e086-7bbc-461c-a565-5b454d748a71",
			},
			Status: "ACTIVE",
		},
		EnrolmentCodes: []string{"abc1234"}}

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(200).JSON(models.IAC{
		IAC:         "abc1234",
		Active:      true,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		QuestionSet: "H1"})

	gock.New("http://localhost:8171").Get("/cases/7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb").Reply(200).JSON(models.Case{
		ID:         "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		BusinessID: "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		CaseGroup: models.CaseGroup{
			ID:                   "aa9c8e93-5cd9-4876-a2d3-78a87b972134",
			CollectionExerciseID: "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		},
	})

	gock.New("http://localhost:8145").Get("/collectionexercises/1010b2f2-8668-498a-afee-3c33cdfe42ea").Reply(200).JSON(models.CollectionExercise{
		ID:       "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		SurveyID: "0752a892-1a60-40a4-8aa3-2599405a8831",
	})

	jsonOut, err := json.Marshal(postReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'POST /respondents', ", err.Error())
	}

	businessRows := mock.NewRows(searchBusinessesQueryColumns)
	businessRows.AddRow("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2")

	mock.ExpectPrepare(searchBusinessesQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)

	mock.ExpectBegin()
	mock.ExpectPrepare(insertQueryRegex).ExpectExec().WithArgs(AnyUUID{}, "CREATED", postReq.Data.Attributes.EmailAddress, postReq.Data.Attributes.FirstName,
		postReq.Data.Attributes.LastName, postReq.Data.Attributes.Telephone, AnyTime{}).WillReturnError(fmt.Errorf("ID already exists"))
	mock.ExpectRollback()
	mock.ExpectClose()

	req := httptest.NewRequest("POST", "/v2/respondents", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'GET /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	assert.Equal(t, "Can't create a respondent with ID be70e086-7bbc-461c-a565-5b454d748a71: ID already exists", errResp.Error)
	assert.True(t, gock.IsDone())
}

func TestPostRespondentsReturns422IfBusinessRespondentCouldntBeInserted(t *testing.T) {
	setup()
	toggleFeature("party.api.post.respondents", true)
	defer gock.Off()
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
				ID:           "be70e086-7bbc-461c-a565-5b454d748a71",
			},
			Status: "ACTIVE",
		},
		EnrolmentCodes: []string{"abc1234"}}

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(200).JSON(models.IAC{
		IAC:         "abc1234",
		Active:      true,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		QuestionSet: "H1"})

	gock.New("http://localhost:8171").Get("/cases/7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb").Reply(200).JSON(models.Case{
		ID:         "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		BusinessID: "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		CaseGroup: models.CaseGroup{
			ID:                   "aa9c8e93-5cd9-4876-a2d3-78a87b972134",
			CollectionExerciseID: "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		},
	})

	gock.New("http://localhost:8145").Get("/collectionexercises/1010b2f2-8668-498a-afee-3c33cdfe42ea").Reply(200).JSON(models.CollectionExercise{
		ID:       "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		SurveyID: "0752a892-1a60-40a4-8aa3-2599405a8831",
	})

	jsonOut, err := json.Marshal(postReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'POST /respondents', ", err.Error())
	}

	businessRows := mock.NewRows(searchBusinessesQueryColumns)
	businessRows.AddRow("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2")

	mock.ExpectPrepare(searchBusinessesQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)

	mock.ExpectBegin()
	mock.ExpectPrepare(insertQueryRegex).ExpectExec().WithArgs(AnyUUID{}, "CREATED", postReq.Data.Attributes.EmailAddress, postReq.Data.Attributes.FirstName,
		postReq.Data.Attributes.LastName, postReq.Data.Attributes.Telephone, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectPrepare(copyQueryRegex).ExpectExec().WithArgs("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2", AnyUUID{}, "ACTIVE", AnyTime{}, AnyTime{}).WillReturnError(fmt.Errorf("Foreign key violation"))
	mock.ExpectRollback()
	mock.ExpectClose()

	req := httptest.NewRequest("POST", "/v2/respondents", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'GET /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	assert.Equal(t, "Can't create a business/respondent link with respondent ID be70e086-7bbc-461c-a565-5b454d748a71 and business ID ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2: Foreign key violation", errResp.Error)
	assert.True(t, gock.IsDone())
}

func TestPostRespondentsReturns422IfBusinessRespondentCouldntBeCommitted(t *testing.T) {
	setup()
	toggleFeature("party.api.post.respondents", true)
	defer gock.Off()
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
				ID:           "be70e086-7bbc-461c-a565-5b454d748a71",
			},
			Status: "ACTIVE",
		},
		EnrolmentCodes: []string{"abc1234"}}

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(200).JSON(models.IAC{
		IAC:         "abc1234",
		Active:      true,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		QuestionSet: "H1"})

	gock.New("http://localhost:8171").Get("/cases/7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb").Reply(200).JSON(models.Case{
		ID:         "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		BusinessID: "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		CaseGroup: models.CaseGroup{
			ID:                   "aa9c8e93-5cd9-4876-a2d3-78a87b972134",
			CollectionExerciseID: "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		},
	})

	gock.New("http://localhost:8145").Get("/collectionexercises/1010b2f2-8668-498a-afee-3c33cdfe42ea").Reply(200).JSON(models.CollectionExercise{
		ID:       "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		SurveyID: "0752a892-1a60-40a4-8aa3-2599405a8831",
	})

	jsonOut, err := json.Marshal(postReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'POST /respondents', ", err.Error())
	}

	businessRows := mock.NewRows(searchBusinessesQueryColumns)
	businessRows.AddRow("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2")

	mock.ExpectPrepare(searchBusinessesQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)

	mock.ExpectBegin()
	mock.ExpectPrepare(insertQueryRegex).ExpectExec().WithArgs(AnyUUID{}, "CREATED", postReq.Data.Attributes.EmailAddress, postReq.Data.Attributes.FirstName,
		postReq.Data.Attributes.LastName, postReq.Data.Attributes.Telephone, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectPrepare(copyQueryRegex).ExpectExec().WithArgs("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2", AnyUUID{}, "ACTIVE", AnyTime{}, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnError(fmt.Errorf("Foreign key violation"))
	mock.ExpectRollback()
	mock.ExpectClose()

	req := httptest.NewRequest("POST", "/v2/respondents", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'GET /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	assert.Equal(t, "Can't commit business/respondent links with respondent ID be70e086-7bbc-461c-a565-5b454d748a71: Foreign key violation", errResp.Error)
	assert.True(t, gock.IsDone())
}

func TestPostRespondentsReturns422IfPendingEnrolmentCouldntBeInserted(t *testing.T) {
	setup()
	toggleFeature("party.api.post.respondents", true)
	defer gock.Off()
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
				ID:           "be70e086-7bbc-461c-a565-5b454d748a71",
			},
			Status: "ACTIVE",
		},
		EnrolmentCodes: []string{"abc1234"}}

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(200).JSON(models.IAC{
		IAC:         "abc1234",
		Active:      true,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		QuestionSet: "H1"})

	gock.New("http://localhost:8171").Get("/cases/7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb").Reply(200).JSON(models.Case{
		ID:         "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		BusinessID: "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		CaseGroup: models.CaseGroup{
			ID:                   "aa9c8e93-5cd9-4876-a2d3-78a87b972134",
			CollectionExerciseID: "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		},
	})

	gock.New("http://localhost:8145").Get("/collectionexercises/1010b2f2-8668-498a-afee-3c33cdfe42ea").Reply(200).JSON(models.CollectionExercise{
		ID:       "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		SurveyID: "0752a892-1a60-40a4-8aa3-2599405a8831",
	})

	jsonOut, err := json.Marshal(postReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'POST /respondents', ", err.Error())
	}

	businessRows := mock.NewRows(searchBusinessesQueryColumns)
	businessRows.AddRow("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2")

	mock.ExpectPrepare(searchBusinessesQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)

	mock.ExpectBegin()
	mock.ExpectPrepare(insertQueryRegex).ExpectExec().WithArgs(AnyUUID{}, "CREATED", postReq.Data.Attributes.EmailAddress, postReq.Data.Attributes.FirstName,
		postReq.Data.Attributes.LastName, postReq.Data.Attributes.Telephone, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectPrepare(copyQueryRegex).ExpectExec().WithArgs("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2", AnyUUID{}, "ACTIVE", AnyTime{}, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectPrepare(copyQueryRegex)
	mock.ExpectPrepare(copyQueryRegex)
	mock.ExpectExec(copyQueryRegex).WithArgs("7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb", AnyUUID{}, "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		"0752a892-1a60-40a4-8aa3-2599405a8831", AnyTime{}).WillReturnError(fmt.Errorf("Foreign key violation"))
	mock.ExpectRollback()
	mock.ExpectClose()

	req := httptest.NewRequest("POST", "/v2/respondents", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'GET /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	assert.Equal(t, "Can't create a Pending Enrolment with respondent ID be70e086-7bbc-461c-a565-5b454d748a71 and business ID ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2: Foreign key violation",
		errResp.Error)
	assert.True(t, gock.IsDone())
}

func TestPostRespondentsReturns422IfPendingEnrolmentCouldntBeCommitted(t *testing.T) {
	setup()
	toggleFeature("party.api.post.respondents", true)
	defer gock.Off()
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
				ID:           "be70e086-7bbc-461c-a565-5b454d748a71",
			},
			Status: "ACTIVE",
		},
		EnrolmentCodes: []string{"abc1234"}}

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(200).JSON(models.IAC{
		IAC:         "abc1234",
		Active:      true,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		QuestionSet: "H1"})

	gock.New("http://localhost:8171").Get("/cases/7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb").Reply(200).JSON(models.Case{
		ID:         "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		BusinessID: "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		CaseGroup: models.CaseGroup{
			ID:                   "aa9c8e93-5cd9-4876-a2d3-78a87b972134",
			CollectionExerciseID: "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		},
	})

	gock.New("http://localhost:8145").Get("/collectionexercises/1010b2f2-8668-498a-afee-3c33cdfe42ea").Reply(200).JSON(models.CollectionExercise{
		ID:       "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		SurveyID: "0752a892-1a60-40a4-8aa3-2599405a8831",
	})

	jsonOut, err := json.Marshal(postReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'POST /respondents', ", err.Error())
	}

	businessRows := mock.NewRows(searchBusinessesQueryColumns)
	businessRows.AddRow("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2")

	mock.ExpectPrepare(searchBusinessesQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)

	mock.ExpectBegin()
	mock.ExpectPrepare(insertQueryRegex).ExpectExec().WithArgs(AnyUUID{}, "CREATED", postReq.Data.Attributes.EmailAddress, postReq.Data.Attributes.FirstName,
		postReq.Data.Attributes.LastName, postReq.Data.Attributes.Telephone, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectPrepare(copyQueryRegex).ExpectExec().WithArgs("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2", AnyUUID{}, "ACTIVE", AnyTime{}, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectPrepare(copyQueryRegex)
	mock.ExpectPrepare(copyQueryRegex)
	mock.ExpectExec(copyQueryRegex).WithArgs("7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb", AnyUUID{}, "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		"0752a892-1a60-40a4-8aa3-2599405a8831", AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WithArgs(AnyUUID{}, "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		"0752a892-1a60-40a4-8aa3-2599405a8831", "PENDING", AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnError(fmt.Errorf("Foreign key violation"))
	mock.ExpectRollback()
	mock.ExpectClose()

	req := httptest.NewRequest("POST", "/v2/respondents", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'GET /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	assert.Equal(t, "Can't commit pending enrolments with respondent ID be70e086-7bbc-461c-a565-5b454d748a71: Foreign key violation", errResp.Error)
	assert.True(t, gock.IsDone())
}

func TestPostRespondentsReturns422IfEnrolmentCouldntBeInserted(t *testing.T) {
	setup()
	toggleFeature("party.api.post.respondents", true)
	defer gock.Off()
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
				ID:           "be70e086-7bbc-461c-a565-5b454d748a71",
			},
			Status: "ACTIVE",
		},
		EnrolmentCodes: []string{"abc1234"}}

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(200).JSON(models.IAC{
		IAC:         "abc1234",
		Active:      true,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		QuestionSet: "H1"})

	gock.New("http://localhost:8171").Get("/cases/7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb").Reply(200).JSON(models.Case{
		ID:         "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		BusinessID: "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		CaseGroup: models.CaseGroup{
			ID:                   "aa9c8e93-5cd9-4876-a2d3-78a87b972134",
			CollectionExerciseID: "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		},
	})

	gock.New("http://localhost:8145").Get("/collectionexercises/1010b2f2-8668-498a-afee-3c33cdfe42ea").Reply(200).JSON(models.CollectionExercise{
		ID:       "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		SurveyID: "0752a892-1a60-40a4-8aa3-2599405a8831",
	})

	jsonOut, err := json.Marshal(postReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'POST /respondents', ", err.Error())
	}

	businessRows := mock.NewRows(searchBusinessesQueryColumns)
	businessRows.AddRow("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2")

	mock.ExpectPrepare(searchBusinessesQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)

	mock.ExpectBegin()
	mock.ExpectPrepare(insertQueryRegex).ExpectExec().WithArgs(AnyUUID{}, "CREATED", postReq.Data.Attributes.EmailAddress, postReq.Data.Attributes.FirstName,
		postReq.Data.Attributes.LastName, postReq.Data.Attributes.Telephone, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectPrepare(copyQueryRegex).ExpectExec().WithArgs("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2", AnyUUID{}, "ACTIVE", AnyTime{}, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectPrepare(copyQueryRegex)
	mock.ExpectPrepare(copyQueryRegex)
	mock.ExpectExec(copyQueryRegex).WithArgs("7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb", AnyUUID{}, "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		"0752a892-1a60-40a4-8aa3-2599405a8831", AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WithArgs(AnyUUID{}, "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		"0752a892-1a60-40a4-8aa3-2599405a8831", "PENDING", AnyTime{}).WillReturnError(fmt.Errorf("Foreign key violation"))
	mock.ExpectRollback()
	mock.ExpectClose()

	req := httptest.NewRequest("POST", "/v2/respondents", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'GET /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	assert.Equal(t, "Can't create an Enrolment with respondent ID be70e086-7bbc-461c-a565-5b454d748a71 and business ID ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2: Foreign key violation",
		errResp.Error)
	assert.True(t, gock.IsDone())
}

func TestPostRespondentsReturns422IfEnrolmentCouldntBeCommitted(t *testing.T) {
	setup()
	toggleFeature("party.api.post.respondents", true)
	defer gock.Off()
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
				ID:           "be70e086-7bbc-461c-a565-5b454d748a71",
			},
			Status: "ACTIVE",
		},
		EnrolmentCodes: []string{"abc1234"}}

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(200).JSON(models.IAC{
		IAC:         "abc1234",
		Active:      true,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		QuestionSet: "H1"})

	gock.New("http://localhost:8171").Get("/cases/7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb").Reply(200).JSON(models.Case{
		ID:         "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		BusinessID: "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		CaseGroup: models.CaseGroup{
			ID:                   "aa9c8e93-5cd9-4876-a2d3-78a87b972134",
			CollectionExerciseID: "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		},
	})

	gock.New("http://localhost:8145").Get("/collectionexercises/1010b2f2-8668-498a-afee-3c33cdfe42ea").Reply(200).JSON(models.CollectionExercise{
		ID:       "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		SurveyID: "0752a892-1a60-40a4-8aa3-2599405a8831",
	})

	jsonOut, err := json.Marshal(postReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'POST /respondents', ", err.Error())
	}

	businessRows := mock.NewRows(searchBusinessesQueryColumns)
	businessRows.AddRow("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2")

	mock.ExpectPrepare(searchBusinessesQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)

	mock.ExpectBegin()
	mock.ExpectPrepare(insertQueryRegex).ExpectExec().WithArgs(AnyUUID{}, "CREATED", postReq.Data.Attributes.EmailAddress, postReq.Data.Attributes.FirstName,
		postReq.Data.Attributes.LastName, postReq.Data.Attributes.Telephone, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectPrepare(copyQueryRegex).ExpectExec().WithArgs("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2", AnyUUID{}, "ACTIVE", AnyTime{}, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectPrepare(copyQueryRegex)
	mock.ExpectPrepare(copyQueryRegex)
	mock.ExpectExec(copyQueryRegex).WithArgs("7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb", AnyUUID{}, "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		"0752a892-1a60-40a4-8aa3-2599405a8831", AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WithArgs(AnyUUID{}, "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		"0752a892-1a60-40a4-8aa3-2599405a8831", "PENDING", AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnError(fmt.Errorf("Foreign key violation"))
	mock.ExpectRollback()
	mock.ExpectClose()

	req := httptest.NewRequest("POST", "/v2/respondents", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'GET /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	assert.Equal(t, "Can't commit enrolments with respondent ID be70e086-7bbc-461c-a565-5b454d748a71: Foreign key violation", errResp.Error)
	assert.True(t, gock.IsDone())
}

func TestPostRespondentsReturns422IfCommitFails(t *testing.T) {
	setup()
	toggleFeature("party.api.post.respondents", true)
	defer gock.Off()
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
				ID:           "be70e086-7bbc-461c-a565-5b454d748a71",
			},
			Status: "ACTIVE",
		},
		EnrolmentCodes: []string{"abc1234"}}

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(200).JSON(models.IAC{
		IAC:         "abc1234",
		Active:      true,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		QuestionSet: "H1"})

	gock.New("http://localhost:8171").Get("/cases/7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb").Reply(200).JSON(models.Case{
		ID:         "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		BusinessID: "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		CaseGroup: models.CaseGroup{
			ID:                   "aa9c8e93-5cd9-4876-a2d3-78a87b972134",
			CollectionExerciseID: "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		},
	})

	gock.New("http://localhost:8145").Get("/collectionexercises/1010b2f2-8668-498a-afee-3c33cdfe42ea").Reply(200).JSON(models.CollectionExercise{
		ID:       "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		SurveyID: "0752a892-1a60-40a4-8aa3-2599405a8831",
	})

	jsonOut, err := json.Marshal(postReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'POST /respondents', ", err.Error())
	}

	businessRows := mock.NewRows(searchBusinessesQueryColumns)
	businessRows.AddRow("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2")

	mock.ExpectPrepare(searchBusinessesQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)

	mock.ExpectBegin()
	mock.ExpectPrepare(insertQueryRegex).ExpectExec().WithArgs(AnyUUID{}, "CREATED", postReq.Data.Attributes.EmailAddress, postReq.Data.Attributes.FirstName,
		postReq.Data.Attributes.LastName, postReq.Data.Attributes.Telephone, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectPrepare(copyQueryRegex).ExpectExec().WithArgs("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2", AnyUUID{}, "ACTIVE", AnyTime{}, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectPrepare(copyQueryRegex)
	mock.ExpectPrepare(copyQueryRegex)
	mock.ExpectExec(copyQueryRegex).WithArgs("7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb", AnyUUID{}, "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		"0752a892-1a60-40a4-8aa3-2599405a8831", AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WithArgs(AnyUUID{}, "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		"0752a892-1a60-40a4-8aa3-2599405a8831", "PENDING", AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit().WillReturnError(fmt.Errorf("Foreign key violation"))
	mock.ExpectRollback()
	mock.ExpectClose()

	req := httptest.NewRequest("POST", "/v2/respondents", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'GET /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	assert.Equal(t, "Can't commit database transaction for respondent ID be70e086-7bbc-461c-a565-5b454d748a71: Foreign key violation", errResp.Error)
	assert.True(t, gock.IsDone())
}

func TestPostRespondentsReturns500WhenDBNotInit(t *testing.T) {
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

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, "Database connection could not be found", errResp.Error)
}

func TestPostRespondentsReturns500WhenDBDown(t *testing.T) {
	setup()
	toggleFeature("party.api.post.respondents", true)
	defer gock.Off()
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

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(200).JSON(models.IAC{
		IAC:         "abc1234",
		Active:      true,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		QuestionSet: "H1"})

	gock.New("http://localhost:8171").Get("/cases/7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb").Reply(200).JSON(models.Case{
		ID:         "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		BusinessID: "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		CaseGroup: models.CaseGroup{
			ID:                   "aa9c8e93-5cd9-4876-a2d3-78a87b972134",
			CollectionExerciseID: "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		},
	})

	gock.New("http://localhost:8145").Get("/collectionexercises/1010b2f2-8668-498a-afee-3c33cdfe42ea").Reply(200).JSON(models.CollectionExercise{
		ID:       "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		SurveyID: "0752a892-1a60-40a4-8aa3-2599405a8831",
	})

	jsonOut, err := json.Marshal(postReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'POST /respondents', ", err.Error())
	}

	mock.ExpectPrepare(searchBusinessesQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnError(fmt.Errorf("Connection refused"))

	req := httptest.NewRequest("POST", "/v2/respondents", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'GET /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, "Error querying DB: Connection refused", errResp.Error)
	assert.True(t, gock.IsDone())
}

func TestPostRespondentsReturns500IfIACCommunicationsFail(t *testing.T) {
	// By not setting up the mock properly, we can effectively test an err in http.Get
	setup()
	toggleFeature("party.api.post.respondents", true)
	defer gock.Off()
	var err error

	db, _, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	gock.New("http://iac-service").Get("/").Reply(200)

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

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Contains(t, errResp.Error, "Couldn't communicate with IAC service:")
}
func TestPostRespondentsReturns500IfCaseCommunicationsFail(t *testing.T) {
	// By not setting up the mock properly, we can effectively test an err in http.Get
	setup()
	toggleFeature("party.api.post.respondents", true)
	defer gock.Off()
	var err error

	db, _, err = sqlmock.New()
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

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(200).JSON(models.IAC{
		IAC:         "abc1234",
		Active:      true,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		QuestionSet: "H1"})

	gock.New("http://case-service").Get("/").Reply(200)

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

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Contains(t, errResp.Error, "Couldn't communicate with Case service:")
}
func TestPostRespondentsReturns500IfCollectionExerciseCommunicationsFail(t *testing.T) {
	// By not setting up the mock properly, we can effectively test an err in http.Get
	setup()
	toggleFeature("party.api.post.respondents", true)
	defer gock.Off()
	var err error

	db, _, err = sqlmock.New()
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

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(200).JSON(models.IAC{
		IAC:         "abc1234",
		Active:      true,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		QuestionSet: "H1"})

	gock.New("http://localhost:8171").Get("/cases/7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb").Reply(200).JSON(models.Case{
		ID:         "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		BusinessID: "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		CaseGroup: models.CaseGroup{
			ID:                   "aa9c8e93-5cd9-4876-a2d3-78a87b972134",
			CollectionExerciseID: "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		},
	})

	gock.New("collection-exercise-service").Get("/").Reply(200)

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

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Contains(t, errResp.Error, "Couldn't communicate with Collection Exercise service:")
}
func TestPostRespondentsReturns500IfDBTransactionCouldntBegin(t *testing.T) {
	setup()
	toggleFeature("party.api.post.respondents", true)
	defer gock.Off()
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
				ID:           "be70e086-7bbc-461c-a565-5b454d748a71",
			},
			Status: "ACTIVE",
		},
		EnrolmentCodes: []string{"abc1234"}}

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(200).JSON(models.IAC{
		IAC:         "abc1234",
		Active:      true,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		QuestionSet: "H1"})

	gock.New("http://localhost:8171").Get("/cases/7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb").Reply(200).JSON(models.Case{
		ID:         "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		BusinessID: "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		CaseGroup: models.CaseGroup{
			ID:                   "aa9c8e93-5cd9-4876-a2d3-78a87b972134",
			CollectionExerciseID: "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		},
	})

	gock.New("http://localhost:8145").Get("/collectionexercises/1010b2f2-8668-498a-afee-3c33cdfe42ea").Reply(200).JSON(models.CollectionExercise{
		ID:       "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		SurveyID: "0752a892-1a60-40a4-8aa3-2599405a8831",
	})

	jsonOut, err := json.Marshal(postReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'POST /respondents', ", err.Error())
	}

	businessRows := mock.NewRows(searchBusinessesQueryColumns)
	businessRows.AddRow("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2")

	mock.ExpectPrepare(searchBusinessesQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)

	mock.ExpectBegin().WillReturnError(fmt.Errorf("Transaction failed"))
	mock.ExpectClose()

	req := httptest.NewRequest("POST", "/v2/respondents", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'GET /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, "Error creating DB transaction: Transaction failed", errResp.Error)
	assert.True(t, gock.IsDone())
}

func TestPostRespondentsReturns500IfInsertRespondentPreparedStatementFails(t *testing.T) {
	setup()
	toggleFeature("party.api.post.respondents", true)
	defer gock.Off()
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
				ID:           "be70e086-7bbc-461c-a565-5b454d748a71",
			},
			Status: "ACTIVE",
		},
		EnrolmentCodes: []string{"abc1234"}}

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(200).JSON(models.IAC{
		IAC:         "abc1234",
		Active:      true,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		QuestionSet: "H1"})

	gock.New("http://localhost:8171").Get("/cases/7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb").Reply(200).JSON(models.Case{
		ID:         "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		BusinessID: "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		CaseGroup: models.CaseGroup{
			ID:                   "aa9c8e93-5cd9-4876-a2d3-78a87b972134",
			CollectionExerciseID: "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		},
	})

	gock.New("http://localhost:8145").Get("/collectionexercises/1010b2f2-8668-498a-afee-3c33cdfe42ea").Reply(200).JSON(models.CollectionExercise{
		ID:       "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		SurveyID: "0752a892-1a60-40a4-8aa3-2599405a8831",
	})

	jsonOut, err := json.Marshal(postReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'POST /respondents', ", err.Error())
	}

	businessRows := mock.NewRows(searchBusinessesQueryColumns)
	businessRows.AddRow("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2")

	mock.ExpectPrepare(searchBusinessesQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)

	mock.ExpectBegin()
	mock.ExpectPrepare(insertQueryRegex).WillReturnError(fmt.Errorf("Syntax error"))
	mock.ExpectClose()

	req := httptest.NewRequest("POST", "/v2/respondents", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'GET /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, "Error creating DB prepared statement: Syntax error", errResp.Error)
	assert.True(t, gock.IsDone())
}

func TestPostRespondentsReturns500IfInsertBusinessRespondentPreparedStatementFails(t *testing.T) {
	setup()
	toggleFeature("party.api.post.respondents", true)
	defer gock.Off()
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
				ID:           "be70e086-7bbc-461c-a565-5b454d748a71",
			},
			Status: "ACTIVE",
		},
		EnrolmentCodes: []string{"abc1234"}}

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(200).JSON(models.IAC{
		IAC:         "abc1234",
		Active:      true,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		QuestionSet: "H1"})

	gock.New("http://localhost:8171").Get("/cases/7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb").Reply(200).JSON(models.Case{
		ID:         "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		BusinessID: "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		CaseGroup: models.CaseGroup{
			ID:                   "aa9c8e93-5cd9-4876-a2d3-78a87b972134",
			CollectionExerciseID: "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		},
	})

	gock.New("http://localhost:8145").Get("/collectionexercises/1010b2f2-8668-498a-afee-3c33cdfe42ea").Reply(200).JSON(models.CollectionExercise{
		ID:       "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		SurveyID: "0752a892-1a60-40a4-8aa3-2599405a8831",
	})

	jsonOut, err := json.Marshal(postReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'POST /respondents', ", err.Error())
	}

	businessRows := mock.NewRows(searchBusinessesQueryColumns)
	businessRows.AddRow("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2")

	mock.ExpectPrepare(searchBusinessesQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)

	mock.ExpectBegin()
	mock.ExpectPrepare(insertQueryRegex).ExpectExec().WithArgs(AnyUUID{}, "CREATED", postReq.Data.Attributes.EmailAddress, postReq.Data.Attributes.FirstName,
		postReq.Data.Attributes.LastName, postReq.Data.Attributes.Telephone, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectPrepare(copyQueryRegex).WillReturnError(fmt.Errorf("Syntax error"))
	mock.ExpectClose()

	req := httptest.NewRequest("POST", "/v2/respondents", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'GET /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, "Error creating DB prepared statement: Syntax error", errResp.Error)
	assert.True(t, gock.IsDone())
}

func TestPostRespondentsReturns500IfInsertPendingEnrolmentPreparedStatementFails(t *testing.T) {
	setup()
	toggleFeature("party.api.post.respondents", true)
	defer gock.Off()
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
				ID:           "be70e086-7bbc-461c-a565-5b454d748a71",
			},
			Status: "ACTIVE",
		},
		EnrolmentCodes: []string{"abc1234"}}

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(200).JSON(models.IAC{
		IAC:         "abc1234",
		Active:      true,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		QuestionSet: "H1"})

	gock.New("http://localhost:8171").Get("/cases/7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb").Reply(200).JSON(models.Case{
		ID:         "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		BusinessID: "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		CaseGroup: models.CaseGroup{
			ID:                   "aa9c8e93-5cd9-4876-a2d3-78a87b972134",
			CollectionExerciseID: "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		},
	})

	gock.New("http://localhost:8145").Get("/collectionexercises/1010b2f2-8668-498a-afee-3c33cdfe42ea").Reply(200).JSON(models.CollectionExercise{
		ID:       "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		SurveyID: "0752a892-1a60-40a4-8aa3-2599405a8831",
	})

	jsonOut, err := json.Marshal(postReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'POST /respondents', ", err.Error())
	}

	businessRows := mock.NewRows(searchBusinessesQueryColumns)
	businessRows.AddRow("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2")

	mock.ExpectPrepare(searchBusinessesQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)

	mock.ExpectBegin()
	mock.ExpectPrepare(insertQueryRegex).ExpectExec().WithArgs(AnyUUID{}, "CREATED", postReq.Data.Attributes.EmailAddress, postReq.Data.Attributes.FirstName,
		postReq.Data.Attributes.LastName, postReq.Data.Attributes.Telephone, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectPrepare(copyQueryRegex).ExpectExec().WithArgs("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2", AnyUUID{}, "ACTIVE", AnyTime{}, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectPrepare(copyQueryRegex).WillReturnError(fmt.Errorf("Syntax error"))
	mock.ExpectClose()

	req := httptest.NewRequest("POST", "/v2/respondents", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'GET /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, "Error creating DB prepared statement: Syntax error", errResp.Error)
	assert.True(t, gock.IsDone())
}

func TestPostRespondentsReturns500IfInsertEnrolmentPreparedStatementFails(t *testing.T) {
	setup()
	toggleFeature("party.api.post.respondents", true)
	defer gock.Off()
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
				ID:           "be70e086-7bbc-461c-a565-5b454d748a71",
			},
			Status: "ACTIVE",
		},
		EnrolmentCodes: []string{"abc1234"}}

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(200).JSON(models.IAC{
		IAC:         "abc1234",
		Active:      true,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		QuestionSet: "H1"})

	gock.New("http://localhost:8171").Get("/cases/7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb").Reply(200).JSON(models.Case{
		ID:         "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		BusinessID: "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		CaseGroup: models.CaseGroup{
			ID:                   "aa9c8e93-5cd9-4876-a2d3-78a87b972134",
			CollectionExerciseID: "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		},
	})

	gock.New("http://localhost:8145").Get("/collectionexercises/1010b2f2-8668-498a-afee-3c33cdfe42ea").Reply(200).JSON(models.CollectionExercise{
		ID:       "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		SurveyID: "0752a892-1a60-40a4-8aa3-2599405a8831",
	})

	jsonOut, err := json.Marshal(postReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'POST /respondents', ", err.Error())
	}

	businessRows := mock.NewRows(searchBusinessesQueryColumns)
	businessRows.AddRow("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2")

	mock.ExpectPrepare(searchBusinessesQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)

	mock.ExpectBegin()
	mock.ExpectPrepare(insertQueryRegex).ExpectExec().WithArgs(AnyUUID{}, "CREATED", postReq.Data.Attributes.EmailAddress, postReq.Data.Attributes.FirstName,
		postReq.Data.Attributes.LastName, postReq.Data.Attributes.Telephone, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectPrepare(copyQueryRegex).ExpectExec().WithArgs("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2", AnyUUID{}, "ACTIVE", AnyTime{}, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectPrepare(copyQueryRegex)
	mock.ExpectPrepare(copyQueryRegex).WillReturnError(fmt.Errorf("Syntax error"))
	mock.ExpectClose()

	req := httptest.NewRequest("POST", "/v2/respondents", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'GET /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, "Error creating DB prepared statement: Syntax error", errResp.Error)
	assert.True(t, gock.IsDone())
}
