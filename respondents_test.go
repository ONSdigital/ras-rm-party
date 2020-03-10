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

var searchRespondentQueryColumns = []string{"id", "email_address", "first_name", "last_name", "telephone", "status", "business_id", "enrolment_status", "survey_id"}
var searchRespondentExistsQueryColumns = []string{"id"}
var searchRespondentForPatchingQueryColumns = []string{"id", "email_address"}
var searchBusinessesQueryColumns = []string{"party_uuid"}
var searchBusinessRespondentsQueryColumns = []string{"business_id"}
var selectQueryRegex = "SELECT (.+) FROM*"
var insertQueryRegex = "INSERT INTO (.+)*"
var copyQueryRegex = "COPY (.+) FROM STDIN"
var deleteQueryRegex = "DELETE FROM (.+)*"
var updateQueryRegex = "UPDATE (.+) SET*"
var postReq = models.PostRespondents{
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
var patchReq = models.PostRespondents{
	Data: models.Respondent{
		Attributes: models.Attributes{
			EmailAddress: "jim@jimbob.com",
			FirstName:    "Bob",
			LastName:     "Boblaw",
			Telephone:    "01234567890",
		},
		Status: "ACTIVE",
		Associations: []models.Association{
			models.Association{
				ID: "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
				Enrolments: []models.Enrolment{
					models.Enrolment{
						SurveyID:        "c43cafd8-ece0-410f-9887-0b0b5eb681fb",
						EnrolmentStatus: "DISABLED",
					},
				},
			},
		},
	},
	EnrolmentCodes: []string{"abc1234"}}

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

	returnRows := mock.NewRows(searchRespondentQueryColumns)
	returnRows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71", "bob@boblaw.com", "Bob", "Boblaw", "01234567890", "ACTIVE", "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2", "ENABLED", "5e237abd-f8dc-4cb0-829e-58d5cef8ca4a")
	returnRows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71", "bob@boblaw.com", "Bob", "Boblaw", "01234567890", "ACTIVE", "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2", "DISABLED", "84bc0d0a-ae32-4fb1-aabc-6de370245d62")
	returnRows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71", "bob@boblaw.com", "Bob", "Boblaw", "01234567890", "ACTIVE", "2711912c-db86-4e1e-9728-fc28db049858", "ENABLED", "ba4274ac-a664-4c3d-8910-18b82a12ce09")
	returnRows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71", "bob@boblaw.com", "Bob", "Boblaw", "01234567890", "ACTIVE", "d4a6c190-50da-4d02-9a78-f4de52d9e6af", "", "")

	mock.ExpectQuery(selectQueryRegex).WillReturnRows(returnRows)
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

	mock.ExpectQuery(selectQueryRegex).WillReturnRows(mock.NewRows(searchRespondentQueryColumns))

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

	mock.ExpectQuery(selectQueryRegex).WillReturnError(fmt.Errorf("Connection refused"))

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

	doublePostReq := models.PostRespondents{
		Data: models.Respondent{
			Attributes: models.Attributes{
				EmailAddress: "bob@boblaw.com",
				FirstName:    "Bob",
				LastName:     "Boblaw",
				Telephone:    "01234567890",
			},
			Status: "ACTIVE",
		},
		EnrolmentCodes: []string{"abc1234", "abc1235"}}

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(200).JSON(models.IAC{
		IAC:         "abc1234",
		Active:      true,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		QuestionSet: "H1"})

	gock.New("http://localhost:8121").Get("/iacs/abc1235").Reply(200).JSON(models.IAC{
		IAC:         "abc1235",
		Active:      true,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "fbb2d260-da57-4607-b829-a2bd434a01dd",
		QuestionSet: "H1"})

	gock.New("http://localhost:8171").Get("/cases/7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb").Reply(200).JSON(models.Case{
		ID:         "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		BusinessID: "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		CaseGroup: models.CaseGroup{
			ID:                   "aa9c8e93-5cd9-4876-a2d3-78a87b972134",
			CollectionExerciseID: "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		},
	})

	gock.New("http://localhost:8171").Get("/cases/fbb2d260-da57-4607-b829-a2bd434a01dd").Reply(200).JSON(models.Case{
		ID:         "fbb2d260-da57-4607-b829-a2bd434a01dd",
		BusinessID: "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		CaseGroup: models.CaseGroup{
			ID:                   "3f8dcbaf-d5d4-415f-bb45-c2cb328320eb",
			CollectionExerciseID: "91b4e876-16af-471e-973e-e3da5ab127bd",
		},
	})

	gock.New("http://localhost:8145").Get("/collectionexercises/1010b2f2-8668-498a-afee-3c33cdfe42ea").Reply(200).JSON(models.CollectionExercise{
		ID:       "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		SurveyID: "0752a892-1a60-40a4-8aa3-2599405a8831",
	})

	gock.New("http://localhost:8145").Get("/collectionexercises/91b4e876-16af-471e-973e-e3da5ab127bd").Reply(200).JSON(models.CollectionExercise{
		ID:       "91b4e876-16af-471e-973e-e3da5ab127bd",
		SurveyID: "c43cafd8-ece0-410f-9887-0b0b5eb681fb",
	})

	gock.New("http://localhost:8121").Put("/abc1234").Reply(200)

	gock.New("http://localhost:8121").Put("/abc1235").Reply(200)

	jsonOut, err := json.Marshal(doublePostReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'POST /respondents', ", err.Error())
	}

	businessRows := mock.NewRows(searchBusinessesQueryColumns)
	businessRows.AddRow("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2")

	mock.ExpectPrepare(selectQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)

	mock.ExpectBegin()
	mock.ExpectPrepare(insertQueryRegex).ExpectExec().WithArgs(AnyUUID{}, "CREATED", postReq.Data.Attributes.EmailAddress, postReq.Data.Attributes.FirstName,
		postReq.Data.Attributes.LastName, postReq.Data.Attributes.Telephone, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectPrepare(copyQueryRegex).ExpectExec().WithArgs("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2", AnyUUID{}, "ACTIVE", AnyTime{}, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectPrepare(copyQueryRegex)
	mock.ExpectPrepare(copyQueryRegex)
	mock.ExpectExec(copyQueryRegex).WithArgs(AnyUUID{}, AnyUUID{}, "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		AnyUUID{}, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WithArgs(AnyUUID{}, "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		AnyUUID{}, "PENDING", AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WithArgs(AnyUUID{}, AnyUUID{}, "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		AnyUUID{}, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WithArgs(AnyUUID{}, "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		AnyUUID{}, "PENDING", AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectClose()

	req := httptest.NewRequest("POST", "/v2/respondents", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var response models.Respondents
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'POST /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusCreated, resp.Code)
	assert.Equal(t, "Bob", response.Data[0].Attributes.FirstName)
	assert.Equal(t, 2, len(response.Data[0].Associations[0].Enrolments))
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

	mock.ExpectPrepare(selectQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)

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

	var response models.Respondents
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'POST /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusCreated, resp.Code)
	assert.Contains(t, logCatcher.String(), "Error deactivating enrolment code abc1234:")
	assert.Equal(t, "Bob", response.Data[0].Attributes.FirstName)
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

	mock.ExpectPrepare(selectQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)

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

	var response models.Respondents
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'POST /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusCreated, resp.Code)
	assert.Contains(t, logCatcher.String(), "Error deactivating enrolment code abc1234: Received status code 404 from IAC service")
	assert.Equal(t, "Bob", response.Data[0].Attributes.FirstName)
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

	wrongPostReq := models.PostRespondents{
		Data: models.Respondent{
			Status: "ACTIVE",
		}}

	jsonOut, err := json.Marshal(wrongPostReq)
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
		t.Fatal("Error decoding JSON response from 'POST /respondents', ", err.Error())
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
		t.Fatal("Error decoding JSON response from 'POST /respondents', ", err.Error())
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
		t.Fatal("Error decoding JSON response from 'POST /respondents', ", err.Error())
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

	mock.ExpectPrepare(selectQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(mock.NewRows(searchBusinessesQueryColumns))

	req := httptest.NewRequest("POST", "/v2/respondents", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'POST /respondents', ", err.Error())
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

	mock.ExpectPrepare(selectQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)

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
		t.Fatal("Error decoding JSON response from 'POST /respondents', ", err.Error())
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

	mock.ExpectPrepare(selectQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)

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
		t.Fatal("Error decoding JSON response from 'POST /respondents', ", err.Error())
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

	mock.ExpectPrepare(selectQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)

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
		t.Fatal("Error decoding JSON response from 'POST /respondents', ", err.Error())
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

	mock.ExpectPrepare(selectQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)

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
		t.Fatal("Error decoding JSON response from 'POST /respondents', ", err.Error())
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

	mock.ExpectPrepare(selectQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)

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
		t.Fatal("Error decoding JSON response from 'POST /respondents', ", err.Error())
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

	mock.ExpectPrepare(selectQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)

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
		t.Fatal("Error decoding JSON response from 'POST /respondents', ", err.Error())
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

	mock.ExpectPrepare(selectQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)

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
		t.Fatal("Error decoding JSON response from 'POST /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	assert.Equal(t, "Can't commit enrolments with respondent ID be70e086-7bbc-461c-a565-5b454d748a71: Foreign key violation", errResp.Error)
	assert.True(t, gock.IsDone())
}

func TestPostRespondentsReturns500WhenDBNotInit(t *testing.T) {
	// It shouldn't be possible to start the app without a DB, but just in case
	setup()
	toggleFeature("party.api.post.respondents", true)
	db = nil

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

	mock.ExpectPrepare(selectQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnError(fmt.Errorf("Connection refused"))

	req := httptest.NewRequest("POST", "/v2/respondents", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'POST /respondents', ", err.Error())
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
		t.Fatal("Error decoding JSON response from 'POST /respondents', ", err.Error())
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
		t.Fatal("Error decoding JSON response from 'POST /respondents', ", err.Error())
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

	mock.ExpectPrepare(selectQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)

	mock.ExpectBegin().WillReturnError(fmt.Errorf("Transaction failed"))
	mock.ExpectClose()

	req := httptest.NewRequest("POST", "/v2/respondents", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'POST /respondents', ", err.Error())
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

	mock.ExpectPrepare(selectQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)

	mock.ExpectBegin()
	mock.ExpectPrepare(insertQueryRegex).WillReturnError(fmt.Errorf("Syntax error"))
	mock.ExpectClose()

	req := httptest.NewRequest("POST", "/v2/respondents", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'POST /respondents', ", err.Error())
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

	mock.ExpectPrepare(selectQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)

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
		t.Fatal("Error decoding JSON response from 'POST /respondents', ", err.Error())
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

	mock.ExpectPrepare(selectQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)

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
		t.Fatal("Error decoding JSON response from 'POST /respondents', ", err.Error())
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

	mock.ExpectPrepare(selectQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)

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
		t.Fatal("Error decoding JSON response from 'POST /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, "Error creating DB prepared statement: Syntax error", errResp.Error)
	assert.True(t, gock.IsDone())
}

func TestPostRespondentsReturns500IfCommitFails(t *testing.T) {
	setup()
	toggleFeature("party.api.post.respondents", true)
	defer gock.Off()
	var mock sqlmock.Sqlmock
	var err error

	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

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

	mock.ExpectPrepare(selectQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)

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
		t.Fatal("Error decoding JSON response from 'POST /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, "Can't commit database transaction for respondent ID be70e086-7bbc-461c-a565-5b454d748a71: Foreign key violation", errResp.Error)
	assert.True(t, gock.IsDone())
}

// DELETE /respondents/{id}
func TestDeleteRespondentsByIDIsFeatureFlagged(t *testing.T) {
	// Assure that it's properly feature flagged away
	setDefaults()
	setup()
	toggleFeature("party.api.delete.respondents.id", false)

	req := httptest.NewRequest("DELETE", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", nil)
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusMethodNotAllowed, resp.Code)
}

func TestDeleteRespondentsByID(t *testing.T) {
	setup()
	toggleFeature("party.api.delete.respondents.id", true)
	var err error
	var mock sqlmock.Sqlmock
	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	rows := mock.NewRows(searchRespondentExistsQueryColumns)
	rows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71")
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(rows)
	mock.ExpectBegin()
	mock.ExpectExec(deleteQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(deleteQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(deleteQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(deleteQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectClose()

	req := httptest.NewRequest("DELETE", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", nil)
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusNoContent, resp.Code)
}

func TestDeleteRespondentsByIDReturns400IfPassedANonUUID(t *testing.T) {
	setup()
	toggleFeature("party.api.delete.respondents.id", true)

	req := httptest.NewRequest("DELETE", "/v2/respondents/abc123", nil)
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err := json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'DELETE /respondents/{id}', ", err.Error())
	}

	assert.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Equal(t, "Not a valid ID: abc123", errResp.Error)
}

func TestDeleteRespondentsByIDReturns401WhenNotAuthed(t *testing.T) {
	setup()
	toggleFeature("party.api.delete.respondents.id", true)

	req := httptest.NewRequest("DELETE", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", nil)
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestDeleteRespondentsByIDReturns404WhenRespondentNotFound(t *testing.T) {
	setup()
	toggleFeature("party.api.delete.respondents.id", true)
	var err error
	var mock sqlmock.Sqlmock
	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(mock.NewRows(searchRespondentExistsQueryColumns))

	req := httptest.NewRequest("DELETE", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", nil)
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'DELETE /respondents/{id}', ", err.Error())
	}

	assert.Equal(t, http.StatusNotFound, resp.Code)
	assert.Equal(t, "Respondent does not exist", errResp.Error)
}

func TestDeleteRespondentsByIDReturns500WhenDBNotInit(t *testing.T) {
	// It shouldn't be possible to start the app without a DB, but just in case
	setup()
	toggleFeature("party.api.delete.respondents.id", true)
	db = nil

	req := httptest.NewRequest("DELETE", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", nil)
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err := json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'DELETE /respondents/{id}', ", err.Error())
	}

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, "Database connection could not be found", errResp.Error)
}

func TestDeleteRespondentsByIDReturns500WhenDBDown(t *testing.T) {
	setup()
	toggleFeature("party.api.delete.respondents.id", true)
	var err error
	var mock sqlmock.Sqlmock
	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	mock.ExpectQuery(selectQueryRegex).WillReturnError(fmt.Errorf("Connection refused"))

	req := httptest.NewRequest("DELETE", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", nil)
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'DELETE /respondents/{id}', ", err.Error())
	}

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, "Error querying DB: Connection refused", errResp.Error)
}

func TestDeleteRespondentsByIDReturns500IfDBTransactionCouldntBegin(t *testing.T) {
	setup()
	toggleFeature("party.api.delete.respondents.id", true)
	defer gock.Off()
	var err error
	var mock sqlmock.Sqlmock
	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	rows := mock.NewRows(searchRespondentExistsQueryColumns)
	rows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71")
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(rows)
	mock.ExpectBegin().WillReturnError(fmt.Errorf("Transaction failed"))

	req := httptest.NewRequest("DELETE", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", nil)
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'DELETE /respondents/{id}', ", err.Error())
	}

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, "Error creating DB transaction: Transaction failed", errResp.Error)
}

func TestDeleteRespondentsByIDReturns500IfDeletingEnrolmentsFails(t *testing.T) {
	setup()
	toggleFeature("party.api.delete.respondents.id", true)
	defer gock.Off()
	var err error
	var mock sqlmock.Sqlmock
	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	rows := mock.NewRows(searchRespondentExistsQueryColumns)
	rows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71")
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(rows)
	mock.ExpectBegin()
	mock.ExpectExec(deleteQueryRegex).WillReturnError(fmt.Errorf("SQL error"))
	mock.ExpectRollback()
	mock.ExpectClose()

	req := httptest.NewRequest("DELETE", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", nil)
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'DELETE /respondents/{id}', ", err.Error())
	}

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, "Error deleting enrolments for respondent ID be70e086-7bbc-461c-a565-5b454d748a71: SQL error", errResp.Error)
}

func TestDeleteRespondentsByIDReturns500IfDeletingBusinessRespondentFails(t *testing.T) {
	setup()
	toggleFeature("party.api.delete.respondents.id", true)
	defer gock.Off()
	var err error
	var mock sqlmock.Sqlmock
	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	rows := mock.NewRows(searchRespondentExistsQueryColumns)
	rows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71")
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(rows)
	mock.ExpectBegin()
	mock.ExpectExec(deleteQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(deleteQueryRegex).WillReturnError(fmt.Errorf("SQL error"))
	mock.ExpectRollback()
	mock.ExpectClose()

	req := httptest.NewRequest("DELETE", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", nil)
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'DELETE /respondents/{id}', ", err.Error())
	}

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, "Error deleting business respondent for respondent ID be70e086-7bbc-461c-a565-5b454d748a71: SQL error", errResp.Error)
}

func TestDeleteRespondentsByIDReturns500IfDeletingPendingEnrolmentsFails(t *testing.T) {
	setup()
	toggleFeature("party.api.delete.respondents.id", true)
	defer gock.Off()
	var err error
	var mock sqlmock.Sqlmock
	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	rows := mock.NewRows(searchRespondentExistsQueryColumns)
	rows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71")
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(rows)
	mock.ExpectBegin()
	mock.ExpectExec(deleteQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(deleteQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(deleteQueryRegex).WillReturnError(fmt.Errorf("SQL error"))
	mock.ExpectRollback()
	mock.ExpectClose()

	req := httptest.NewRequest("DELETE", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", nil)
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'DELETE /respondents/{id}', ", err.Error())
	}

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, "Error deleting pending enrolments for respondent ID be70e086-7bbc-461c-a565-5b454d748a71: SQL error", errResp.Error)
}

func TestDeleteRespondentsByIDReturns500IfDeletingRespondentFails(t *testing.T) {
	setup()
	toggleFeature("party.api.delete.respondents.id", true)
	defer gock.Off()
	var err error
	var mock sqlmock.Sqlmock
	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	rows := mock.NewRows(searchRespondentExistsQueryColumns)
	rows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71")
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(rows)
	mock.ExpectBegin()
	mock.ExpectExec(deleteQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(deleteQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(deleteQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(deleteQueryRegex).WillReturnError(fmt.Errorf("SQL error"))
	mock.ExpectRollback()
	mock.ExpectClose()

	req := httptest.NewRequest("DELETE", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", nil)
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'DELETE /respondents/{id}', ", err.Error())
	}

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, "Error deleting respondent record for respondent ID be70e086-7bbc-461c-a565-5b454d748a71: SQL error", errResp.Error)
}

func TestDeleteRespondentsByIDReturns500IfTransactionCommitFails(t *testing.T) {
	setup()
	toggleFeature("party.api.delete.respondents.id", true)
	defer gock.Off()
	var err error
	var mock sqlmock.Sqlmock
	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	rows := mock.NewRows(searchRespondentExistsQueryColumns)
	rows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71")
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(rows)
	mock.ExpectBegin()
	mock.ExpectExec(deleteQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(deleteQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(deleteQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(deleteQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit().WillReturnError(fmt.Errorf("Table locked"))
	mock.ExpectRollback()
	mock.ExpectClose()

	req := httptest.NewRequest("DELETE", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", nil)
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'DELETE /respondents/{id}', ", err.Error())
	}

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, "Can't commit transaction for respondent ID be70e086-7bbc-461c-a565-5b454d748a71: Table locked", errResp.Error)
}

// GET /respondents/id
func TestGetRespondentsByIDIsFeatureFlagged(t *testing.T) {
	// Assure that it's properly feature flagged away
	setDefaults()
	setup()
	toggleFeature("party.api.get.respondents.id", false)

	req := httptest.NewRequest("GET", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", nil)
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusMethodNotAllowed, resp.Code)
}

func TestGetRespondentsByID(t *testing.T) {
	setDefaults()
	setup()
	toggleFeature("party.api.get.respondents.id", true)
	var err error
	var mock sqlmock.Sqlmock
	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	returnRows := mock.NewRows(searchRespondentQueryColumns)
	returnRows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71", "bob@boblaw.com", "Bob", "Boblaw", "01234567890", "ACTIVE", "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2", "ENABLED", "5e237abd-f8dc-4cb0-829e-58d5cef8ca4a")
	returnRows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71", "bob@boblaw.com", "Bob", "Boblaw", "01234567890", "ACTIVE", "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2", "DISABLED", "84bc0d0a-ae32-4fb1-aabc-6de370245d62")
	returnRows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71", "bob@boblaw.com", "Bob", "Boblaw", "01234567890", "ACTIVE", "2711912c-db86-4e1e-9728-fc28db049858", "ENABLED", "ba4274ac-a664-4c3d-8910-18b82a12ce09")
	returnRows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71", "bob@boblaw.com", "Bob", "Boblaw", "01234567890", "ACTIVE", "d4a6c190-50da-4d02-9a78-f4de52d9e6af", "", "")

	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(returnRows)

	req := httptest.NewRequest("GET", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", nil)
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var respondent models.Respondents
	err = json.NewDecoder(resp.Body).Decode(&respondent)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'GET /respondents/{id}', ", err.Error())
	}

	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, 1, len(respondent.Data))
	assert.Equal(t, "be70e086-7bbc-461c-a565-5b454d748a71", respondent.Data[0].Attributes.ID)
	assert.Equal(t, 3, len(respondent.Data[0].Associations))
	assert.Equal(t, 2, len(respondent.Data[0].Associations[0].Enrolments))
	assert.Equal(t, 1, len(respondent.Data[0].Associations[1].Enrolments))
	assert.Equal(t, 0, len(respondent.Data[0].Associations[2].Enrolments))
}

func TestGetRespondentsByIDReturns400IfPassedANonUUID(t *testing.T) {
	setDefaults()
	setup()
	toggleFeature("party.api.get.respondents.id", true)

	req := httptest.NewRequest("GET", "/v2/respondents/abc123", nil)
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err := json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'GET /respondents/{id}', ", err.Error())
	}

	assert.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Equal(t, "Not a valid ID: abc123", errResp.Error)
}

func TestGetRespondentsByIDReturns401WhenNotAuthed(t *testing.T) {
	setDefaults()
	setup()
	toggleFeature("party.api.get.respondents.id", true)

	req := httptest.NewRequest("GET", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", nil)
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestGetRespondentsByIDReturns404WhenNoResults(t *testing.T) {
	setDefaults()
	setup()
	toggleFeature("party.api.get.respondents.id", true)
	var err error
	var mock sqlmock.Sqlmock
	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	mock.ExpectQuery(selectQueryRegex).WillReturnRows(sqlmock.NewRows(searchRespondentQueryColumns))

	req := httptest.NewRequest("GET", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", nil)
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'GET /respondents/{id}', ", err.Error())
	}

	assert.Equal(t, http.StatusNotFound, resp.Code)
	assert.Equal(t, "No respondent found for ID be70e086-7bbc-461c-a565-5b454d748a71", errResp.Error)
}

func TestGetRespondentsByIDReturns500WhenDBNotInit(t *testing.T) {
	setDefaults()
	setup()
	toggleFeature("party.api.get.respondents.id", true)

	db = nil

	req := httptest.NewRequest("GET", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", nil)
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err := json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'GET /respondents/{id}', ", err.Error())
	}

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, "Database connection could not be found", errResp.Error)
}

func TestGetRespondentsByIDReturns500WhenDBDown(t *testing.T) {
	setup()
	toggleFeature("party.api.get.respondents.id", true)

	var mock sqlmock.Sqlmock
	var err error

	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	mock.ExpectQuery(selectQueryRegex).WillReturnError(fmt.Errorf("Connection refused"))

	req := httptest.NewRequest("GET", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", nil)
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'GET /respondents/{id}', ", err.Error())
	}

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, "Error querying DB: Connection refused", errResp.Error)
}

// PATCH /respondents/id
func TestPatchRespondentsByIDIsFeatureFlagged(t *testing.T) {
	// Assure that it's properly feature flagged away
	setDefaults()
	setup()
	toggleFeature("party.api.patch.respondents.id", false)

	req := httptest.NewRequest("PATCH", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", nil)
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusMethodNotAllowed, resp.Code)
}

func TestPatchRespondentsByID(t *testing.T) {
	setDefaults()
	setup()
	toggleFeature("party.api.patch.respondents.id", true)
	var err error
	var mock sqlmock.Sqlmock

	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	doublePatchReq := models.PostRespondents{
		Data: models.Respondent{
			Attributes: models.Attributes{
				EmailAddress: "bob@boblaw.com",
				FirstName:    "Bob",
				LastName:     "Boblaw",
				Telephone:    "01234567890",
			},
			Status: "ACTIVE",
			Associations: []models.Association{
				models.Association{
					ID: "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
					Enrolments: []models.Enrolment{
						models.Enrolment{
							SurveyID:        "c43cafd8-ece0-410f-9887-0b0b5eb681fb",
							EnrolmentStatus: "DISABLED",
						},
					},
				},
			},
		},
		EnrolmentCodes: []string{"abc1234", "abc1235"}}

	jsonOut, err := json.Marshal(doublePatchReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'PATCH /respondents/{id}', ", err.Error())
	}

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(200).JSON(models.IAC{
		IAC:         "abc1234",
		Active:      true,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		QuestionSet: "H1"})

	gock.New("http://localhost:8121").Get("/iacs/abc1235").Reply(200).JSON(models.IAC{
		IAC:         "abc1235",
		Active:      true,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "fbb2d260-da57-4607-b829-a2bd434a01dd",
		QuestionSet: "H1"})

	gock.New("http://localhost:8171").Get("/cases/7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb").Reply(200).JSON(models.Case{
		ID:         "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		BusinessID: "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		CaseGroup: models.CaseGroup{
			ID:                   "aa9c8e93-5cd9-4876-a2d3-78a87b972134",
			CollectionExerciseID: "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		},
	})

	gock.New("http://localhost:8171").Get("/cases/fbb2d260-da57-4607-b829-a2bd434a01dd").Reply(200).JSON(models.Case{
		ID:         "fbb2d260-da57-4607-b829-a2bd434a01dd",
		BusinessID: "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		CaseGroup: models.CaseGroup{
			ID:                   "3f8dcbaf-d5d4-415f-bb45-c2cb328320eb",
			CollectionExerciseID: "91b4e876-16af-471e-973e-e3da5ab127bd",
		},
	})

	gock.New("http://localhost:8145").Get("/collectionexercises/1010b2f2-8668-498a-afee-3c33cdfe42ea").Reply(200).JSON(models.CollectionExercise{
		ID:       "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		SurveyID: "0752a892-1a60-40a4-8aa3-2599405a8831",
	})

	gock.New("http://localhost:8145").Get("/collectionexercises/91b4e876-16af-471e-973e-e3da5ab127bd").Reply(200).JSON(models.CollectionExercise{
		ID:       "91b4e876-16af-471e-973e-e3da5ab127bd",
		SurveyID: "c43cafd8-ece0-410f-9887-0b0b5eb681fb",
	})

	respondentRows := mock.NewRows(searchRespondentForPatchingQueryColumns)
	respondentRows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71", "bob@boblaw.com")

	businessRespondentRows := mock.NewRows(searchBusinessRespondentsQueryColumns)
	businessRespondentRows.AddRow("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2")

	mock.ExpectBegin()
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(respondentRows)
	mock.ExpectExec(updateQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(businessRespondentRows)
	mock.ExpectPrepare(copyQueryRegex)
	mock.ExpectPrepare(copyQueryRegex)
	mock.ExpectExec(copyQueryRegex).WithArgs(AnyUUID{}, "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		AnyUUID{}, "PENDING", AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WithArgs(AnyUUID{}, AnyUUID{}, "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		AnyUUID{}, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WithArgs(AnyUUID{}, "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		AnyUUID{}, "PENDING", AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WithArgs(AnyUUID{}, AnyUUID{}, "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		AnyUUID{}, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectPrepare(updateQueryRegex).ExpectExec().WithArgs("DISABLED", "be70e086-7bbc-461c-a565-5b454d748a71", "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		"c43cafd8-ece0-410f-9887-0b0b5eb681fb").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectClose()

	req := httptest.NewRequest("PATCH", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	assert.True(t, gock.IsDone())
}

func TestPatchRespondentsByIDReturns400IfPassedANonUUID(t *testing.T) {
	setDefaults()
	setup()
	toggleFeature("party.api.patch.respondents.id", true)

	req := httptest.NewRequest("PATCH", "/v2/respondents/abc123", nil)
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err := json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'PATCH /respondents/{id}', ", err.Error())
	}

	assert.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Equal(t, "Not a valid ID: abc123", errResp.Error)
}

func TestPatchRespondentsByIDReturns400IfBadJSON(t *testing.T) {
	setDefaults()
	setup()
	toggleFeature("party.api.patch.respondents.id", true)

	req := httptest.NewRequest("PATCH", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", strings.NewReader("{nonsense: true}"))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err := json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'PATCH /respondents/{id}', ", err.Error())
	}

	assert.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Equal(t, "Invalid JSON", errResp.Error)
}

func TestPatchRespondentsByIDReturns400IfIDChanged(t *testing.T) {
	setDefaults()
	setup()
	toggleFeature("party.api.patch.respondents.id", true)

	idChangePatchReq := models.PostRespondents{
		Data: models.Respondent{
			Attributes: models.Attributes{
				EmailAddress: "bob@boblaw.com",
				FirstName:    "Bob",
				LastName:     "Boblaw",
				Telephone:    "01234567890",
				ID:           "aaaaaaaa-7bbc-461c-a565-5b454d748a71",
			},
			Status: "ACTIVE",
		},
		EnrolmentCodes: []string{"abc1234", "abc1235"}}

	jsonOut, err := json.Marshal(idChangePatchReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'PATCH /respondents/{id}', ", err.Error())
	}

	req := httptest.NewRequest("PATCH", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'PATCH /respondents/{id}', ", err.Error())
	}

	assert.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Equal(t, "ID must not be changed", errResp.Error)
}

func TestPatchRespondentsByIDReturns400IfBadRespondentStatus(t *testing.T) {
	setDefaults()
	setup()
	toggleFeature("party.api.patch.respondents.id", true)
	var err error
	var mock sqlmock.Sqlmock

	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	badStatusPatchReq := models.PostRespondents{
		Data: models.Respondent{
			Attributes: models.Attributes{
				EmailAddress: "jim@jimbob.com",
				FirstName:    "Bob",
				LastName:     "Boblaw",
				Telephone:    "01234567890",
			},
			Status: "WRONG",
		},
		EnrolmentCodes: []string{"abc1234", "abc1235"}}

	jsonOut, err := json.Marshal(badStatusPatchReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'PATCH /respondents/{id}', ", err.Error())
	}

	respondentRows := mock.NewRows(searchRespondentForPatchingQueryColumns)
	respondentRows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71", "bob@boblaw.com")

	mock.ExpectBegin()
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(respondentRows)
	mock.ExpectQuery(selectQueryRegex).WithArgs("jim@jimbob.com").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow("0"))
	mock.ExpectRollback()
	mock.ExpectClose()

	req := httptest.NewRequest("PATCH", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'PATCH /respondents/{id}', ", err.Error())
	}

	assert.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Equal(t, "Invalid respondent status provided: WRONG", errResp.Error)
}

func TestPatchRespondentsByIDReturns401WhenNotAuthed(t *testing.T) {
	setDefaults()
	setup()
	toggleFeature("party.api.patch.respondents.id", true)

	req := httptest.NewRequest("PATCH", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", nil)
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusUnauthorized, resp.Code)
}

func TestPatchRespondentsByIDReturns404IfRespondentNotFound(t *testing.T) {
	setDefaults()
	setup()
	toggleFeature("party.api.patch.respondents.id", true)
	var err error
	var mock sqlmock.Sqlmock

	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	jsonOut, err := json.Marshal(patchReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'PATCH /respondents/{id}', ", err.Error())
	}

	mock.ExpectBegin()
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(sqlmock.NewRows(searchRespondentForPatchingQueryColumns))
	mock.ExpectClose()

	req := httptest.NewRequest("PATCH", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'PATCH /respondents/{id}', ", err.Error())
	}

	assert.Equal(t, http.StatusNotFound, resp.Code)
	assert.Equal(t, "Respondent does not exist", errResp.Error)
}

func TestPatchRespondentsByIDReturns404IfEnrolmentCodeNotFound(t *testing.T) {
	setup()
	toggleFeature("party.api.patch.respondents.id", true)
	defer gock.Off()

	var err error
	var mock sqlmock.Sqlmock

	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	jsonOut, err := json.Marshal(patchReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'PATCH /respondents/{id}', ", err.Error())
	}

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(404)

	respondentRows := mock.NewRows(searchRespondentForPatchingQueryColumns)
	respondentRows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71", "bob@boblaw.com")

	mock.ExpectBegin()
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(respondentRows)
	mock.ExpectQuery(selectQueryRegex).WithArgs("jim@jimbob.com").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow("0"))
	mock.ExpectExec(updateQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectRollback()
	mock.ExpectClose()

	req := httptest.NewRequest("PATCH", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'PATCH /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusNotFound, resp.Code)
	assert.Equal(t, "Enrolment code not found: abc1234", errResp.Error)
	assert.True(t, gock.IsDone())
}

func TestPatchRespondentsByIDReturns404IfCaseNotFound(t *testing.T) {
	setup()
	toggleFeature("party.api.patch.respondents.id", true)
	defer gock.Off()

	var err error
	var mock sqlmock.Sqlmock

	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	jsonOut, err := json.Marshal(patchReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'PATCH /respondents/{id}', ", err.Error())
	}

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(200).JSON(models.IAC{
		IAC:         "abc1234",
		Active:      true,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		QuestionSet: "H1"})

	gock.New("http://localhost:8171").Get("/cases/7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb").Reply(404)

	respondentRows := mock.NewRows(searchRespondentForPatchingQueryColumns)
	respondentRows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71", "bob@boblaw.com")

	mock.ExpectBegin()
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(respondentRows)
	mock.ExpectQuery(selectQueryRegex).WithArgs("jim@jimbob.com").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow("0"))
	mock.ExpectExec(updateQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectRollback()
	mock.ExpectClose()

	req := httptest.NewRequest("PATCH", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'PATCH /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusNotFound, resp.Code)
	assert.Equal(t, "Case not found for enrolment code: abc1234", errResp.Error)
	assert.True(t, gock.IsDone())
}

func TestPatchRespondentsByIDReturns404IfCollectionExerciseNotFound(t *testing.T) {
	setup()
	toggleFeature("party.api.patch.respondents.id", true)
	defer gock.Off()

	var err error
	var mock sqlmock.Sqlmock

	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	jsonOut, err := json.Marshal(patchReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'PATCH /respondents/{id}', ", err.Error())
	}

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

	respondentRows := mock.NewRows(searchRespondentForPatchingQueryColumns)
	respondentRows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71", "bob@boblaw.com")

	mock.ExpectBegin()
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(respondentRows)
	mock.ExpectQuery(selectQueryRegex).WithArgs("jim@jimbob.com").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow("0"))
	mock.ExpectExec(updateQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectRollback()
	mock.ExpectClose()

	req := httptest.NewRequest("PATCH", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'PATCH /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusNotFound, resp.Code)
	assert.Equal(t, "Collection Exercise not found for enrolment code: abc1234", errResp.Error)
	assert.True(t, gock.IsDone())
}

func TestPatchRespondentsByIDReturns404IfEnrolmentCouldntBeFoundFromJSON(t *testing.T) {
	setDefaults()
	setup()
	toggleFeature("party.api.patch.respondents.id", true)
	var err error
	var mock sqlmock.Sqlmock

	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	jsonOut, err := json.Marshal(patchReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'PATCH /respondents/{id}', ", err.Error())
	}

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(200).JSON(models.IAC{
		IAC:         "abc1234",
		Active:      true,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		QuestionSet: "H1"})

	gock.New("http://localhost:8171").Get("/cases/7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb").Reply(200).JSON(models.Case{
		ID:         "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		BusinessID: "aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2",
		CaseGroup: models.CaseGroup{
			ID:                   "aa9c8e93-5cd9-4876-a2d3-78a87b972134",
			CollectionExerciseID: "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		},
	})

	gock.New("http://localhost:8145").Get("/collectionexercises/1010b2f2-8668-498a-afee-3c33cdfe42ea").Reply(200).JSON(models.CollectionExercise{
		ID:       "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		SurveyID: "0752a892-1a60-40a4-8aa3-2599405a8831",
	})

	respondentRows := mock.NewRows(searchRespondentForPatchingQueryColumns)
	respondentRows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71", "bob@boblaw.com")

	businessRespondentRows := mock.NewRows(searchBusinessRespondentsQueryColumns)
	businessRespondentRows.AddRow("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2")

	businessRows := mock.NewRows(searchBusinessRespondentsQueryColumns)
	businessRows.AddRow("aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2")

	mock.ExpectBegin()
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(respondentRows)
	mock.ExpectQuery(selectQueryRegex).WithArgs("jim@jimbob.com").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow("0"))
	mock.ExpectExec(updateQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(businessRespondentRows)
	mock.ExpectPrepare(selectQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)
	mock.ExpectPrepare(copyQueryRegex).ExpectExec().WithArgs("aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2", "be70e086-7bbc-461c-a565-5b454d748a71", "ACTIVE", AnyTime{}, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectPrepare(copyQueryRegex)
	mock.ExpectPrepare(copyQueryRegex)
	mock.ExpectExec(copyQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71", "aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2",
		"0752a892-1a60-40a4-8aa3-2599405a8831", "PENDING", AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WithArgs(AnyUUID{}, AnyUUID{}, "aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2",
		AnyUUID{}, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectPrepare(updateQueryRegex).ExpectExec().WithArgs("DISABLED", "be70e086-7bbc-461c-a565-5b454d748a71", "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		"c43cafd8-ece0-410f-9887-0b0b5eb681fb").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()
	mock.ExpectClose()

	req := httptest.NewRequest("PATCH", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'PATCH /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusNotFound, resp.Code)
	assert.Equal(t, "Can't find enrolment to update for respondent ID be70e086-7bbc-461c-a565-5b454d748a71 and survey ID c43cafd8-ece0-410f-9887-0b0b5eb681fb", errResp.Error)
	assert.True(t, gock.IsDone())
}

func TestPatchRespondentsByIDReturns409IfEmailNotUnique(t *testing.T) {
	setDefaults()
	setup()
	toggleFeature("party.api.patch.respondents.id", true)
	var err error
	var mock sqlmock.Sqlmock

	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	jsonOut, err := json.Marshal(patchReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'PATCH /respondents/{id}', ", err.Error())
	}

	respondentRows := mock.NewRows(searchRespondentForPatchingQueryColumns)
	respondentRows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71", "bob@boblaw.com")

	mock.ExpectBegin()
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(respondentRows)
	mock.ExpectQuery(selectQueryRegex).WithArgs("jim@jimbob.com").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow("1"))
	mock.ExpectClose()

	req := httptest.NewRequest("PATCH", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'PATCH /respondents/{id}', ", err.Error())
	}

	assert.Equal(t, http.StatusConflict, resp.Code)
	assert.Equal(t, "New email address already in use", errResp.Error)
}

func TestPatchRespondentsByIDReturns422IfUpdateRespondentPreparedStatementFails(t *testing.T) {
	setDefaults()
	setup()
	toggleFeature("party.api.patch.respondents.id", true)
	var err error
	var mock sqlmock.Sqlmock

	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	jsonOut, err := json.Marshal(patchReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'PATCH /respondents/{id}', ", err.Error())
	}

	respondentRows := mock.NewRows(searchRespondentForPatchingQueryColumns)
	respondentRows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71", "bob@boblaw.com")

	mock.ExpectBegin()
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(respondentRows)
	mock.ExpectQuery(selectQueryRegex).WithArgs("jim@jimbob.com").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow("0"))
	mock.ExpectExec(updateQueryRegex).WillReturnError(fmt.Errorf("Connection refused"))
	mock.ExpectRollback()
	mock.ExpectClose()

	req := httptest.NewRequest("PATCH", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'PATCH /respondents/{id}', ", err.Error())
	}

	assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	assert.Equal(t, "Can't update respondent for ID be70e086-7bbc-461c-a565-5b454d748a71: Connection refused", errResp.Error)
}

func TestPatchRespondentsByIDReturns422IfEnrolmentCodeInactive(t *testing.T) {
	setup()
	toggleFeature("party.api.patch.respondents.id", true)
	defer gock.Off()

	var err error
	var mock sqlmock.Sqlmock

	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	jsonOut, err := json.Marshal(patchReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'PATCH /respondents/{id}', ", err.Error())
	}

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(200).JSON(models.IAC{
		IAC:         "abc1234",
		Active:      false,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		QuestionSet: "H1"})

	respondentRows := mock.NewRows(searchRespondentForPatchingQueryColumns)
	respondentRows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71", "bob@boblaw.com")

	mock.ExpectBegin()
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(respondentRows)
	mock.ExpectQuery(selectQueryRegex).WithArgs("jim@jimbob.com").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow("0"))
	mock.ExpectExec(updateQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectRollback()
	mock.ExpectClose()

	req := httptest.NewRequest("PATCH", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'PATCH /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	assert.Equal(t, "Enrolment code inactive: abc1234", errResp.Error)
	assert.True(t, gock.IsDone())
}

func TestPatchRespondentsByIDReturns422IfBusinessNotFoundToAssociate(t *testing.T) {
	setDefaults()
	setup()
	toggleFeature("party.api.patch.respondents.id", true)
	var err error
	var mock sqlmock.Sqlmock

	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	jsonOut, err := json.Marshal(patchReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'PATCH /respondents/{id}', ", err.Error())
	}

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(200).JSON(models.IAC{
		IAC:         "abc1234",
		Active:      true,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "26e06e05-a12a-4768-a846-aeb77708026e",
		QuestionSet: "H1"})

	gock.New("http://localhost:8171").Get("/cases/26e06e05-a12a-4768-a846-aeb77708026e").Reply(200).JSON(models.Case{
		ID:         "26e06e05-a12a-4768-a846-aeb77708026e",
		BusinessID: "aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2",
		CaseGroup: models.CaseGroup{
			ID:                   "5704c2b5-21c0-43b5-b3c5-699e4bd09bce",
			CollectionExerciseID: "49624f9f-4955-41cc-917b-d9353d75677c",
		},
	})

	gock.New("http://localhost:8145").Get("/collectionexercises/49624f9f-4955-41cc-917b-d9353d75677c").Reply(200).JSON(models.CollectionExercise{
		ID:       "49624f9f-4955-41cc-917b-d9353d75677c",
		SurveyID: "ab4e763f-2bdf-4da3-b9d9-fcfb6175418d",
	})

	respondentRows := mock.NewRows(searchRespondentForPatchingQueryColumns)
	respondentRows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71", "bob@boblaw.com")

	businessRespondentRows := mock.NewRows(searchBusinessRespondentsQueryColumns)
	businessRespondentRows.AddRow("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2")

	mock.ExpectBegin()
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(respondentRows)
	mock.ExpectQuery(selectQueryRegex).WithArgs("jim@jimbob.com").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow("0"))
	mock.ExpectExec(updateQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(businessRespondentRows)
	mock.ExpectPrepare(selectQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2"})).
		WillReturnRows(sqlmock.NewRows(searchBusinessRespondentsQueryColumns))
	mock.ExpectRollback()
	mock.ExpectClose()

	req := httptest.NewRequest("PATCH", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'PATCH /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	assert.Contains(t, errResp.Error, "Can't associate with the business for enrolment code: abc1234")
	assert.True(t, gock.IsDone())
}

func TestPatchRespondentsByIDReturns422IfBusinessRespondentCouldntBeInserted(t *testing.T) {
	setDefaults()
	setup()
	toggleFeature("party.api.patch.respondents.id", true)
	var err error
	var mock sqlmock.Sqlmock

	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	jsonOut, err := json.Marshal(patchReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'PATCH /respondents/{id}', ", err.Error())
	}

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(200).JSON(models.IAC{
		IAC:         "abc1234",
		Active:      true,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		QuestionSet: "H1"})

	gock.New("http://localhost:8171").Get("/cases/7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb").Reply(200).JSON(models.Case{
		ID:         "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		BusinessID: "aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2",
		CaseGroup: models.CaseGroup{
			ID:                   "aa9c8e93-5cd9-4876-a2d3-78a87b972134",
			CollectionExerciseID: "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		},
	})

	gock.New("http://localhost:8145").Get("/collectionexercises/1010b2f2-8668-498a-afee-3c33cdfe42ea").Reply(200).JSON(models.CollectionExercise{
		ID:       "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		SurveyID: "0752a892-1a60-40a4-8aa3-2599405a8831",
	})

	respondentRows := mock.NewRows(searchRespondentForPatchingQueryColumns)
	respondentRows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71", "bob@boblaw.com")

	businessRespondentRows := mock.NewRows(searchBusinessRespondentsQueryColumns)
	businessRespondentRows.AddRow("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2")

	businessRows := mock.NewRows(searchBusinessRespondentsQueryColumns)
	businessRows.AddRow("aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2")

	mock.ExpectBegin()
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(respondentRows)
	mock.ExpectQuery(selectQueryRegex).WithArgs("jim@jimbob.com").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow("0"))
	mock.ExpectExec(updateQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(businessRespondentRows)
	mock.ExpectPrepare(selectQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)
	mock.ExpectPrepare(copyQueryRegex).ExpectExec().WithArgs("aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2", AnyUUID{}, "ACTIVE", AnyTime{}, AnyTime{}).WillReturnError(fmt.Errorf("Connection refused"))
	mock.ExpectRollback()
	mock.ExpectClose()

	req := httptest.NewRequest("PATCH", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'PATCH /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	assert.Equal(t, "Can't create a business/respondent link with respondent ID be70e086-7bbc-461c-a565-5b454d748a71 and business ID aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2: Connection refused", errResp.Error)
	assert.True(t, gock.IsDone())
}

func TestPatchRespondentsByIDReturns422IfBusinessRespondentCouldntBeCommitted(t *testing.T) {
	setDefaults()
	setup()
	toggleFeature("party.api.patch.respondents.id", true)
	var err error
	var mock sqlmock.Sqlmock

	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	jsonOut, err := json.Marshal(patchReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'PATCH /respondents/{id}', ", err.Error())
	}

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(200).JSON(models.IAC{
		IAC:         "abc1234",
		Active:      true,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		QuestionSet: "H1"})

	gock.New("http://localhost:8171").Get("/cases/7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb").Reply(200).JSON(models.Case{
		ID:         "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		BusinessID: "aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2",
		CaseGroup: models.CaseGroup{
			ID:                   "aa9c8e93-5cd9-4876-a2d3-78a87b972134",
			CollectionExerciseID: "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		},
	})

	gock.New("http://localhost:8145").Get("/collectionexercises/1010b2f2-8668-498a-afee-3c33cdfe42ea").Reply(200).JSON(models.CollectionExercise{
		ID:       "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		SurveyID: "0752a892-1a60-40a4-8aa3-2599405a8831",
	})

	respondentRows := mock.NewRows(searchRespondentForPatchingQueryColumns)
	respondentRows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71", "bob@boblaw.com")

	businessRespondentRows := mock.NewRows(searchBusinessRespondentsQueryColumns)
	businessRespondentRows.AddRow("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2")

	businessRows := mock.NewRows(searchBusinessRespondentsQueryColumns)
	businessRows.AddRow("aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2")

	mock.ExpectBegin()
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(respondentRows)
	mock.ExpectQuery(selectQueryRegex).WithArgs("jim@jimbob.com").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow("0"))
	mock.ExpectExec(updateQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(businessRespondentRows)
	mock.ExpectPrepare(selectQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)
	mock.ExpectPrepare(copyQueryRegex).ExpectExec().WithArgs("aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2", AnyUUID{}, "ACTIVE", AnyTime{}, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnError(fmt.Errorf("Connection refused"))
	mock.ExpectRollback()
	mock.ExpectClose()

	req := httptest.NewRequest("PATCH", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'PATCH /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	assert.Equal(t, "Can't commit business/respondent links with respondent ID be70e086-7bbc-461c-a565-5b454d748a71: Connection refused", errResp.Error)
	assert.True(t, gock.IsDone())
}

func TestPatchRespondentsByIDReturns422IfEnrolmentCouldntBeInsertedForIAC(t *testing.T) {
	setDefaults()
	setup()
	toggleFeature("party.api.patch.respondents.id", true)
	var err error
	var mock sqlmock.Sqlmock

	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	jsonOut, err := json.Marshal(patchReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'PATCH /respondents/{id}', ", err.Error())
	}

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(200).JSON(models.IAC{
		IAC:         "abc1234",
		Active:      true,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		QuestionSet: "H1"})

	gock.New("http://localhost:8171").Get("/cases/7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb").Reply(200).JSON(models.Case{
		ID:         "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		BusinessID: "aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2",
		CaseGroup: models.CaseGroup{
			ID:                   "aa9c8e93-5cd9-4876-a2d3-78a87b972134",
			CollectionExerciseID: "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		},
	})

	gock.New("http://localhost:8145").Get("/collectionexercises/1010b2f2-8668-498a-afee-3c33cdfe42ea").Reply(200).JSON(models.CollectionExercise{
		ID:       "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		SurveyID: "0752a892-1a60-40a4-8aa3-2599405a8831",
	})

	respondentRows := mock.NewRows(searchRespondentForPatchingQueryColumns)
	respondentRows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71", "bob@boblaw.com")

	businessRespondentRows := mock.NewRows(searchBusinessRespondentsQueryColumns)
	businessRespondentRows.AddRow("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2")

	businessRows := mock.NewRows(searchBusinessRespondentsQueryColumns)
	businessRows.AddRow("aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2")

	mock.ExpectBegin()
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(respondentRows)
	mock.ExpectQuery(selectQueryRegex).WithArgs("jim@jimbob.com").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow("0"))
	mock.ExpectExec(updateQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(businessRespondentRows)
	mock.ExpectPrepare(selectQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)
	mock.ExpectPrepare(copyQueryRegex).ExpectExec().WithArgs("aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2", "be70e086-7bbc-461c-a565-5b454d748a71", "ACTIVE", AnyTime{}, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectPrepare(copyQueryRegex)
	mock.ExpectPrepare(copyQueryRegex)
	mock.ExpectExec(copyQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71", "aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2",
		"0752a892-1a60-40a4-8aa3-2599405a8831", "PENDING", AnyTime{}).WillReturnError(fmt.Errorf("Foreign key violation"))
	mock.ExpectRollback()
	mock.ExpectClose()

	req := httptest.NewRequest("PATCH", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'PATCH /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	assert.Equal(t, "Can't create an Enrolment with respondent ID be70e086-7bbc-461c-a565-5b454d748a71 and business ID aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2: Foreign key violation", errResp.Error)
	assert.True(t, gock.IsDone())
}

func TestPatchRespondentsByIDReturns422IfPendingEnrolmentCouldntBeInserted(t *testing.T) {
	setDefaults()
	setup()
	toggleFeature("party.api.patch.respondents.id", true)
	var err error
	var mock sqlmock.Sqlmock

	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	jsonOut, err := json.Marshal(patchReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'PATCH /respondents/{id}', ", err.Error())
	}

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(200).JSON(models.IAC{
		IAC:         "abc1234",
		Active:      true,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		QuestionSet: "H1"})

	gock.New("http://localhost:8171").Get("/cases/7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb").Reply(200).JSON(models.Case{
		ID:         "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		BusinessID: "aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2",
		CaseGroup: models.CaseGroup{
			ID:                   "aa9c8e93-5cd9-4876-a2d3-78a87b972134",
			CollectionExerciseID: "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		},
	})

	gock.New("http://localhost:8145").Get("/collectionexercises/1010b2f2-8668-498a-afee-3c33cdfe42ea").Reply(200).JSON(models.CollectionExercise{
		ID:       "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		SurveyID: "0752a892-1a60-40a4-8aa3-2599405a8831",
	})

	respondentRows := mock.NewRows(searchRespondentForPatchingQueryColumns)
	respondentRows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71", "bob@boblaw.com")

	businessRespondentRows := mock.NewRows(searchBusinessRespondentsQueryColumns)
	businessRespondentRows.AddRow("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2")

	businessRows := mock.NewRows(searchBusinessRespondentsQueryColumns)
	businessRows.AddRow("aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2")

	mock.ExpectBegin()
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(respondentRows)
	mock.ExpectQuery(selectQueryRegex).WithArgs("jim@jimbob.com").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow("0"))
	mock.ExpectExec(updateQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(businessRespondentRows)
	mock.ExpectPrepare(selectQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)
	mock.ExpectPrepare(copyQueryRegex).ExpectExec().WithArgs("aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2", "be70e086-7bbc-461c-a565-5b454d748a71", "ACTIVE", AnyTime{}, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectPrepare(copyQueryRegex)
	mock.ExpectPrepare(copyQueryRegex)
	mock.ExpectExec(copyQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71", "aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2",
		"0752a892-1a60-40a4-8aa3-2599405a8831", "PENDING", AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WithArgs(AnyUUID{}, AnyUUID{}, "aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2",
		AnyUUID{}, AnyTime{}).WillReturnError(fmt.Errorf("Foreign key violation"))
	mock.ExpectRollback()
	mock.ExpectClose()

	req := httptest.NewRequest("PATCH", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'PATCH /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	assert.Equal(t, "Can't create a Pending Enrolment with respondent ID be70e086-7bbc-461c-a565-5b454d748a71 and business ID aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2: Foreign key violation", errResp.Error)
	assert.True(t, gock.IsDone())
}

func TestPatchRespondentsByIDReturns422IfEnrolmentCouldntBeCommitted(t *testing.T) {
	setDefaults()
	setup()
	toggleFeature("party.api.patch.respondents.id", true)
	var err error
	var mock sqlmock.Sqlmock

	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	jsonOut, err := json.Marshal(patchReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'PATCH /respondents/{id}', ", err.Error())
	}

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(200).JSON(models.IAC{
		IAC:         "abc1234",
		Active:      true,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		QuestionSet: "H1"})

	gock.New("http://localhost:8171").Get("/cases/7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb").Reply(200).JSON(models.Case{
		ID:         "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		BusinessID: "aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2",
		CaseGroup: models.CaseGroup{
			ID:                   "aa9c8e93-5cd9-4876-a2d3-78a87b972134",
			CollectionExerciseID: "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		},
	})

	gock.New("http://localhost:8145").Get("/collectionexercises/1010b2f2-8668-498a-afee-3c33cdfe42ea").Reply(200).JSON(models.CollectionExercise{
		ID:       "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		SurveyID: "0752a892-1a60-40a4-8aa3-2599405a8831",
	})

	respondentRows := mock.NewRows(searchRespondentForPatchingQueryColumns)
	respondentRows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71", "bob@boblaw.com")

	businessRespondentRows := mock.NewRows(searchBusinessRespondentsQueryColumns)
	businessRespondentRows.AddRow("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2")

	businessRows := mock.NewRows(searchBusinessRespondentsQueryColumns)
	businessRows.AddRow("aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2")

	mock.ExpectBegin()
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(respondentRows)
	mock.ExpectQuery(selectQueryRegex).WithArgs("jim@jimbob.com").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow("0"))
	mock.ExpectExec(updateQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(businessRespondentRows)
	mock.ExpectPrepare(selectQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)
	mock.ExpectPrepare(copyQueryRegex).ExpectExec().WithArgs("aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2", "be70e086-7bbc-461c-a565-5b454d748a71", "ACTIVE", AnyTime{}, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectPrepare(copyQueryRegex)
	mock.ExpectPrepare(copyQueryRegex)
	mock.ExpectExec(copyQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71", "aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2",
		"0752a892-1a60-40a4-8aa3-2599405a8831", "PENDING", AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WithArgs(AnyUUID{}, AnyUUID{}, "aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2",
		AnyUUID{}, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnError(fmt.Errorf("Foreign key violation"))
	mock.ExpectRollback()
	mock.ExpectClose()

	req := httptest.NewRequest("PATCH", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'PATCH /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	assert.Equal(t, "Can't commit enrolments with respondent ID be70e086-7bbc-461c-a565-5b454d748a71: Foreign key violation", errResp.Error)
	assert.True(t, gock.IsDone())
}

func TestPatchRespondentsByIDReturns422IfPendingEnrolmentCouldntBeCommitted(t *testing.T) {
	setDefaults()
	setup()
	toggleFeature("party.api.patch.respondents.id", true)
	var err error
	var mock sqlmock.Sqlmock

	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	jsonOut, err := json.Marshal(patchReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'PATCH /respondents/{id}', ", err.Error())
	}

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(200).JSON(models.IAC{
		IAC:         "abc1234",
		Active:      true,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		QuestionSet: "H1"})

	gock.New("http://localhost:8171").Get("/cases/7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb").Reply(200).JSON(models.Case{
		ID:         "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		BusinessID: "aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2",
		CaseGroup: models.CaseGroup{
			ID:                   "aa9c8e93-5cd9-4876-a2d3-78a87b972134",
			CollectionExerciseID: "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		},
	})

	gock.New("http://localhost:8145").Get("/collectionexercises/1010b2f2-8668-498a-afee-3c33cdfe42ea").Reply(200).JSON(models.CollectionExercise{
		ID:       "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		SurveyID: "0752a892-1a60-40a4-8aa3-2599405a8831",
	})

	respondentRows := mock.NewRows(searchRespondentForPatchingQueryColumns)
	respondentRows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71", "bob@boblaw.com")

	businessRespondentRows := mock.NewRows(searchBusinessRespondentsQueryColumns)
	businessRespondentRows.AddRow("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2")

	businessRows := mock.NewRows(searchBusinessRespondentsQueryColumns)
	businessRows.AddRow("aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2")

	mock.ExpectBegin()
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(respondentRows)
	mock.ExpectQuery(selectQueryRegex).WithArgs("jim@jimbob.com").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow("0"))
	mock.ExpectExec(updateQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(businessRespondentRows)
	mock.ExpectPrepare(selectQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)
	mock.ExpectPrepare(copyQueryRegex).ExpectExec().WithArgs("aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2", "be70e086-7bbc-461c-a565-5b454d748a71", "ACTIVE", AnyTime{}, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectPrepare(copyQueryRegex)
	mock.ExpectPrepare(copyQueryRegex)
	mock.ExpectExec(copyQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71", "aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2",
		"0752a892-1a60-40a4-8aa3-2599405a8831", "PENDING", AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WithArgs(AnyUUID{}, AnyUUID{}, "aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2",
		AnyUUID{}, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnError(fmt.Errorf("Foreign key violation"))
	mock.ExpectRollback()
	mock.ExpectClose()

	req := httptest.NewRequest("PATCH", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'PATCH /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	assert.Equal(t, "Can't commit pending enrolments with respondent ID be70e086-7bbc-461c-a565-5b454d748a71: Foreign key violation", errResp.Error)
	assert.True(t, gock.IsDone())
}

func TestPatchRespondentsByIDReturns422IfEnrolmentCouldntBeUpdatedFromJSON(t *testing.T) {
	setDefaults()
	setup()
	toggleFeature("party.api.patch.respondents.id", true)
	var err error
	var mock sqlmock.Sqlmock

	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	jsonOut, err := json.Marshal(patchReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'PATCH /respondents/{id}', ", err.Error())
	}

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(200).JSON(models.IAC{
		IAC:         "abc1234",
		Active:      true,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		QuestionSet: "H1"})

	gock.New("http://localhost:8171").Get("/cases/7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb").Reply(200).JSON(models.Case{
		ID:         "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		BusinessID: "aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2",
		CaseGroup: models.CaseGroup{
			ID:                   "aa9c8e93-5cd9-4876-a2d3-78a87b972134",
			CollectionExerciseID: "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		},
	})

	gock.New("http://localhost:8145").Get("/collectionexercises/1010b2f2-8668-498a-afee-3c33cdfe42ea").Reply(200).JSON(models.CollectionExercise{
		ID:       "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		SurveyID: "0752a892-1a60-40a4-8aa3-2599405a8831",
	})

	respondentRows := mock.NewRows(searchRespondentForPatchingQueryColumns)
	respondentRows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71", "bob@boblaw.com")

	businessRespondentRows := mock.NewRows(searchBusinessRespondentsQueryColumns)
	businessRespondentRows.AddRow("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2")

	businessRows := mock.NewRows(searchBusinessRespondentsQueryColumns)
	businessRows.AddRow("aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2")

	mock.ExpectBegin()
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(respondentRows)
	mock.ExpectQuery(selectQueryRegex).WithArgs("jim@jimbob.com").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow("0"))
	mock.ExpectExec(updateQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(businessRespondentRows)
	mock.ExpectPrepare(selectQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)
	mock.ExpectPrepare(copyQueryRegex).ExpectExec().WithArgs("aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2", "be70e086-7bbc-461c-a565-5b454d748a71", "ACTIVE", AnyTime{}, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectPrepare(copyQueryRegex)
	mock.ExpectPrepare(copyQueryRegex)
	mock.ExpectExec(copyQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71", "aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2",
		"0752a892-1a60-40a4-8aa3-2599405a8831", "PENDING", AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WithArgs(AnyUUID{}, AnyUUID{}, "aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2",
		AnyUUID{}, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectPrepare(updateQueryRegex).ExpectExec().WithArgs("DISABLED", "be70e086-7bbc-461c-a565-5b454d748a71", "ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2",
		"c43cafd8-ece0-410f-9887-0b0b5eb681fb").WillReturnError(fmt.Errorf("Foreign key violation"))
	mock.ExpectRollback()
	mock.ExpectClose()

	req := httptest.NewRequest("PATCH", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'PATCH /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	assert.Equal(t, "Can't update an Enrolment with respondent ID be70e086-7bbc-461c-a565-5b454d748a71 and business ID ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2: Foreign key violation",
		errResp.Error)
	assert.True(t, gock.IsDone())
}

func TestPatchRespondentsByIDReturns500WhenDBNotInit(t *testing.T) {
	setDefaults()
	setup()
	toggleFeature("party.api.patch.respondents.id", true)

	db = nil

	jsonOut, err := json.Marshal(patchReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'PATCH /respondents/{id}', ", err.Error())
	}

	req := httptest.NewRequest("PATCH", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'PATCH /respondents/{id}', ", err.Error())
	}

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, "Database connection could not be found", errResp.Error)
}

func TestPatchRespondentsByIDReturns500IfDBTransactionCouldntBegin(t *testing.T) {
	setDefaults()
	setup()
	toggleFeature("party.api.patch.respondents.id", true)
	var err error
	var mock sqlmock.Sqlmock

	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	jsonOut, err := json.Marshal(patchReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'PATCH /respondents/{id}', ", err.Error())
	}

	mock.ExpectBegin().WillReturnError(fmt.Errorf("Transaction failed"))
	mock.ExpectClose()

	req := httptest.NewRequest("PATCH", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'PATCH /respondents/{id}', ", err.Error())
	}

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, "Error creating DB transaction: Transaction failed", errResp.Error)
}

func TestPatchRespondentsByIDReturns500IfRetrievingRespondentFails(t *testing.T) {
	setDefaults()
	setup()
	toggleFeature("party.api.patch.respondents.id", true)
	var err error
	var mock sqlmock.Sqlmock

	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	jsonOut, err := json.Marshal(patchReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'PATCH /respondents/{id}', ", err.Error())
	}

	mock.ExpectBegin()
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnError(fmt.Errorf("Connection refused"))
	mock.ExpectClose()

	req := httptest.NewRequest("PATCH", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'PATCH /respondents/{id}', ", err.Error())
	}

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, "Error querying DB: Connection refused", errResp.Error)
}

func TestPatchRespondentsByIDReturns500IfCheckingEmailUniquenessFails(t *testing.T) {
	setDefaults()
	setup()
	toggleFeature("party.api.patch.respondents.id", true)
	var err error
	var mock sqlmock.Sqlmock

	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	jsonOut, err := json.Marshal(patchReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'PATCH /respondents/{id}', ", err.Error())
	}

	respondentRows := mock.NewRows(searchRespondentForPatchingQueryColumns)
	respondentRows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71", "bob@boblaw.com")

	mock.ExpectBegin()
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(respondentRows)
	mock.ExpectQuery(selectQueryRegex).WithArgs("jim@jimbob.com").WillReturnError(fmt.Errorf("Connection refused"))
	mock.ExpectClose()

	req := httptest.NewRequest("PATCH", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'PATCH /respondents/{id}', ", err.Error())
	}

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, "Error querying DB: Connection refused", errResp.Error)
}

func TestPatchRespondentsByIDReturns500IfIACCommunicationsFail(t *testing.T) {
	// By not setting up the mock properly, we can effectively test an err in http.Get
	setup()
	toggleFeature("party.api.patch.respondents.id", true)
	defer gock.Off()

	var err error
	var mock sqlmock.Sqlmock

	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	jsonOut, err := json.Marshal(patchReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'PATCH /respondents/{id}', ", err.Error())
	}

	gock.New("http://iac-service").Get("/").Reply(200)

	respondentRows := mock.NewRows(searchRespondentForPatchingQueryColumns)
	respondentRows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71", "bob@boblaw.com")

	mock.ExpectBegin()
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(respondentRows)
	mock.ExpectQuery(selectQueryRegex).WithArgs("jim@jimbob.com").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow("0"))
	mock.ExpectExec(updateQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectRollback()
	mock.ExpectClose()

	req := httptest.NewRequest("PATCH", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'PATCH /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Contains(t, errResp.Error, "Couldn't communicate with IAC service:")
}

func TestPatchRespondentsByIDReturns500IfCaseCommunicationsFail(t *testing.T) {
	// By not setting up the mock properly, we can effectively test an err in http.Get
	setup()
	toggleFeature("party.api.patch.respondents.id", true)
	defer gock.Off()

	var err error
	var mock sqlmock.Sqlmock

	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	jsonOut, err := json.Marshal(patchReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'PATCH /respondents/{id}', ", err.Error())
	}

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(200).JSON(models.IAC{
		IAC:         "abc1234",
		Active:      true,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		QuestionSet: "H1"})

	gock.New("http://case-service").Get("/").Reply(200)

	respondentRows := mock.NewRows(searchRespondentForPatchingQueryColumns)
	respondentRows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71", "bob@boblaw.com")

	mock.ExpectBegin()
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(respondentRows)
	mock.ExpectQuery(selectQueryRegex).WithArgs("jim@jimbob.com").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow("0"))
	mock.ExpectExec(updateQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectRollback()
	mock.ExpectClose()

	req := httptest.NewRequest("PATCH", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'PATCH /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Contains(t, errResp.Error, "Couldn't communicate with Case service:")
}

func TestPatchRespondentsByIDReturns500IfCollectionExerciseCommunicationsFail(t *testing.T) {
	// By not setting up the mock properly, we can effectively test an err in http.Get
	setup()
	toggleFeature("party.api.patch.respondents.id", true)
	defer gock.Off()

	var err error
	var mock sqlmock.Sqlmock

	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	jsonOut, err := json.Marshal(patchReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'PATCH /respondents/{id}', ", err.Error())
	}

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

	respondentRows := mock.NewRows(searchRespondentForPatchingQueryColumns)
	respondentRows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71", "bob@boblaw.com")

	mock.ExpectBegin()
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(respondentRows)
	mock.ExpectQuery(selectQueryRegex).WithArgs("jim@jimbob.com").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow("0"))
	mock.ExpectExec(updateQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectRollback()
	mock.ExpectClose()

	req := httptest.NewRequest("PATCH", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'PATCH /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Contains(t, errResp.Error, "Couldn't communicate with Collection Exercise service:")
}

func TestPatchRespondentsByIDReturns500IfRetrievingBusinessRespondentsFails(t *testing.T) {
	setDefaults()
	setup()
	toggleFeature("party.api.patch.respondents.id", true)
	var err error
	var mock sqlmock.Sqlmock

	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	jsonOut, err := json.Marshal(patchReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'PATCH /respondents/{id}', ", err.Error())
	}

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

	respondentRows := mock.NewRows(searchRespondentForPatchingQueryColumns)
	respondentRows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71", "bob@boblaw.com")

	mock.ExpectBegin()
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(respondentRows)
	mock.ExpectQuery(selectQueryRegex).WithArgs("jim@jimbob.com").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow("0"))
	mock.ExpectExec(updateQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnError(fmt.Errorf("Connection refused"))
	mock.ExpectRollback()
	mock.ExpectClose()

	req := httptest.NewRequest("PATCH", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'PATCH /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Contains(t, errResp.Error, "Can't retrieve existing business associations for respondent ID be70e086-7bbc-461c-a565-5b454d748a71: Connection refused")
	assert.True(t, gock.IsDone())
}

func TestPatchRespondentsByIDReturns500IfRetrievingBusinessesFails(t *testing.T) {
	setDefaults()
	setup()
	toggleFeature("party.api.patch.respondents.id", true)
	var err error
	var mock sqlmock.Sqlmock

	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	jsonOut, err := json.Marshal(patchReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'PATCH /respondents/{id}', ", err.Error())
	}

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(200).JSON(models.IAC{
		IAC:         "abc1234",
		Active:      true,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		QuestionSet: "H1"})

	gock.New("http://localhost:8171").Get("/cases/7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb").Reply(200).JSON(models.Case{
		ID:         "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		BusinessID: "aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2",
		CaseGroup: models.CaseGroup{
			ID:                   "aa9c8e93-5cd9-4876-a2d3-78a87b972134",
			CollectionExerciseID: "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		},
	})

	gock.New("http://localhost:8145").Get("/collectionexercises/1010b2f2-8668-498a-afee-3c33cdfe42ea").Reply(200).JSON(models.CollectionExercise{
		ID:       "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		SurveyID: "0752a892-1a60-40a4-8aa3-2599405a8831",
	})

	respondentRows := mock.NewRows(searchRespondentForPatchingQueryColumns)
	respondentRows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71", "bob@boblaw.com")

	businessRespondentRows := mock.NewRows(searchBusinessRespondentsQueryColumns)
	businessRespondentRows.AddRow("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2")

	mock.ExpectBegin()
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(respondentRows)
	mock.ExpectQuery(selectQueryRegex).WithArgs("jim@jimbob.com").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow("0"))
	mock.ExpectExec(updateQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(businessRespondentRows)
	mock.ExpectPrepare(selectQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnError(fmt.Errorf("Connection refused"))
	mock.ExpectRollback()
	mock.ExpectClose()

	req := httptest.NewRequest("PATCH", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'PATCH /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Contains(t, errResp.Error, "Error querying DB: Connection refused")
	assert.True(t, gock.IsDone())
}
func TestPatchRespondentsByIDReturns500IfInsertBusinessRespondentPreparedStatementFails(t *testing.T) {
	setDefaults()
	setup()
	toggleFeature("party.api.patch.respondents.id", true)
	var err error
	var mock sqlmock.Sqlmock

	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	jsonOut, err := json.Marshal(patchReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'PATCH /respondents/{id}', ", err.Error())
	}

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(200).JSON(models.IAC{
		IAC:         "abc1234",
		Active:      true,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		QuestionSet: "H1"})

	gock.New("http://localhost:8171").Get("/cases/7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb").Reply(200).JSON(models.Case{
		ID:         "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		BusinessID: "aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2",
		CaseGroup: models.CaseGroup{
			ID:                   "aa9c8e93-5cd9-4876-a2d3-78a87b972134",
			CollectionExerciseID: "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		},
	})

	gock.New("http://localhost:8145").Get("/collectionexercises/1010b2f2-8668-498a-afee-3c33cdfe42ea").Reply(200).JSON(models.CollectionExercise{
		ID:       "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		SurveyID: "0752a892-1a60-40a4-8aa3-2599405a8831",
	})

	respondentRows := mock.NewRows(searchRespondentForPatchingQueryColumns)
	respondentRows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71", "bob@boblaw.com")

	businessRespondentRows := mock.NewRows(searchBusinessRespondentsQueryColumns)
	businessRespondentRows.AddRow("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2")

	businessRows := mock.NewRows(searchBusinessRespondentsQueryColumns)
	businessRows.AddRow("aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2")

	mock.ExpectBegin()
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(respondentRows)
	mock.ExpectQuery(selectQueryRegex).WithArgs("jim@jimbob.com").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow("0"))
	mock.ExpectExec(updateQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(businessRespondentRows)
	mock.ExpectPrepare(selectQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)
	mock.ExpectPrepare(copyQueryRegex).WillReturnError(fmt.Errorf("Syntax error"))
	mock.ExpectRollback()
	mock.ExpectClose()

	req := httptest.NewRequest("PATCH", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'PATCH /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, "Error creating DB prepared statement: Syntax error", errResp.Error)
	assert.True(t, gock.IsDone())
}

func TestPatchRespondentsByIDReturns500IfInsertEnrolmentFromIACPreparedStatementFails(t *testing.T) {
	setDefaults()
	setup()
	toggleFeature("party.api.patch.respondents.id", true)
	var err error
	var mock sqlmock.Sqlmock

	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	jsonOut, err := json.Marshal(patchReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'PATCH /respondents/{id}', ", err.Error())
	}

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(200).JSON(models.IAC{
		IAC:         "abc1234",
		Active:      true,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		QuestionSet: "H1"})

	gock.New("http://localhost:8171").Get("/cases/7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb").Reply(200).JSON(models.Case{
		ID:         "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		BusinessID: "aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2",
		CaseGroup: models.CaseGroup{
			ID:                   "aa9c8e93-5cd9-4876-a2d3-78a87b972134",
			CollectionExerciseID: "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		},
	})

	gock.New("http://localhost:8145").Get("/collectionexercises/1010b2f2-8668-498a-afee-3c33cdfe42ea").Reply(200).JSON(models.CollectionExercise{
		ID:       "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		SurveyID: "0752a892-1a60-40a4-8aa3-2599405a8831",
	})

	respondentRows := mock.NewRows(searchRespondentForPatchingQueryColumns)
	respondentRows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71", "bob@boblaw.com")

	businessRespondentRows := mock.NewRows(searchBusinessRespondentsQueryColumns)
	businessRespondentRows.AddRow("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2")

	businessRows := mock.NewRows(searchBusinessRespondentsQueryColumns)
	businessRows.AddRow("aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2")

	mock.ExpectBegin()
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(respondentRows)
	mock.ExpectQuery(selectQueryRegex).WithArgs("jim@jimbob.com").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow("0"))
	mock.ExpectExec(updateQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(businessRespondentRows)
	mock.ExpectPrepare(selectQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)
	mock.ExpectPrepare(copyQueryRegex).ExpectExec().WithArgs("aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2", AnyUUID{}, "ACTIVE", AnyTime{}, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectPrepare(copyQueryRegex).WillReturnError(fmt.Errorf("Syntax error"))
	mock.ExpectRollback()
	mock.ExpectClose()

	req := httptest.NewRequest("PATCH", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'PATCH /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, "Error creating DB prepared statement: Syntax error", errResp.Error)
	assert.True(t, gock.IsDone())
}

func TestPatchRespondentsByIDReturns500IfInsertPendingEnrolmentPreparedStatementFails(t *testing.T) {
	setDefaults()
	setup()
	toggleFeature("party.api.patch.respondents.id", true)
	var err error
	var mock sqlmock.Sqlmock

	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	jsonOut, err := json.Marshal(patchReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'PATCH /respondents/{id}', ", err.Error())
	}

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(200).JSON(models.IAC{
		IAC:         "abc1234",
		Active:      true,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		QuestionSet: "H1"})

	gock.New("http://localhost:8171").Get("/cases/7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb").Reply(200).JSON(models.Case{
		ID:         "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		BusinessID: "aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2",
		CaseGroup: models.CaseGroup{
			ID:                   "aa9c8e93-5cd9-4876-a2d3-78a87b972134",
			CollectionExerciseID: "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		},
	})

	gock.New("http://localhost:8145").Get("/collectionexercises/1010b2f2-8668-498a-afee-3c33cdfe42ea").Reply(200).JSON(models.CollectionExercise{
		ID:       "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		SurveyID: "0752a892-1a60-40a4-8aa3-2599405a8831",
	})

	respondentRows := mock.NewRows(searchRespondentForPatchingQueryColumns)
	respondentRows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71", "bob@boblaw.com")

	businessRespondentRows := mock.NewRows(searchBusinessRespondentsQueryColumns)
	businessRespondentRows.AddRow("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2")

	businessRows := mock.NewRows(searchBusinessRespondentsQueryColumns)
	businessRows.AddRow("aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2")

	mock.ExpectBegin()
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(respondentRows)
	mock.ExpectQuery(selectQueryRegex).WithArgs("jim@jimbob.com").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow("0"))
	mock.ExpectExec(updateQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(businessRespondentRows)
	mock.ExpectPrepare(selectQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)
	mock.ExpectPrepare(copyQueryRegex).ExpectExec().WithArgs("aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2", AnyUUID{}, "ACTIVE", AnyTime{}, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectPrepare(copyQueryRegex)
	mock.ExpectPrepare(copyQueryRegex).WillReturnError(fmt.Errorf("Syntax error"))
	mock.ExpectRollback()
	mock.ExpectClose()

	req := httptest.NewRequest("PATCH", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'PATCH /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, "Error creating DB prepared statement: Syntax error", errResp.Error)
	assert.True(t, gock.IsDone())
}

func TestPatchRespondentsByIDReturns500IfInsertEnrolmentFromJSONPreparedStatementFails(t *testing.T) {
	setDefaults()
	setup()
	toggleFeature("party.api.patch.respondents.id", true)
	var err error
	var mock sqlmock.Sqlmock

	db, mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("Error setting up an SQL mock")
	}

	jsonOut, err := json.Marshal(patchReq)
	if err != nil {
		t.Fatal("Error encoding JSON request body for 'PATCH /respondents/{id}', ", err.Error())
	}

	gock.New("http://localhost:8121").Get("/iacs/abc1234").Reply(200).JSON(models.IAC{
		IAC:         "abc1234",
		Active:      true,
		LastUsed:    "2017-05-15T10:00:00Z",
		CaseID:      "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		QuestionSet: "H1"})

	gock.New("http://localhost:8171").Get("/cases/7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb").Reply(200).JSON(models.Case{
		ID:         "7bc5d41b-0549-40b3-ba76-42f6d4cf3fdb",
		BusinessID: "aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2",
		CaseGroup: models.CaseGroup{
			ID:                   "aa9c8e93-5cd9-4876-a2d3-78a87b972134",
			CollectionExerciseID: "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		},
	})

	gock.New("http://localhost:8145").Get("/collectionexercises/1010b2f2-8668-498a-afee-3c33cdfe42ea").Reply(200).JSON(models.CollectionExercise{
		ID:       "1010b2f2-8668-498a-afee-3c33cdfe42ea",
		SurveyID: "0752a892-1a60-40a4-8aa3-2599405a8831",
	})

	respondentRows := mock.NewRows(searchRespondentForPatchingQueryColumns)
	respondentRows.AddRow("be70e086-7bbc-461c-a565-5b454d748a71", "bob@boblaw.com")

	businessRespondentRows := mock.NewRows(searchBusinessRespondentsQueryColumns)
	businessRespondentRows.AddRow("ba02fad7-ae27-45c6-ab0f-c8cd9a48ebc2")

	businessRows := mock.NewRows(searchBusinessRespondentsQueryColumns)
	businessRows.AddRow("aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2")

	mock.ExpectBegin()
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(respondentRows)
	mock.ExpectQuery(selectQueryRegex).WithArgs("jim@jimbob.com").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow("0"))
	mock.ExpectExec(updateQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(selectQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71").WillReturnRows(businessRespondentRows)
	mock.ExpectPrepare(selectQueryRegex).ExpectQuery().WithArgs(pq.Array([]string{"aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2"})).WillReturnRows(businessRows)
	mock.ExpectPrepare(copyQueryRegex).ExpectExec().WithArgs("aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2", "be70e086-7bbc-461c-a565-5b454d748a71", "ACTIVE", AnyTime{}, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectPrepare(copyQueryRegex)
	mock.ExpectPrepare(copyQueryRegex)
	mock.ExpectExec(copyQueryRegex).WithArgs("be70e086-7bbc-461c-a565-5b454d748a71", "aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2",
		"0752a892-1a60-40a4-8aa3-2599405a8831", "PENDING", AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WithArgs(AnyUUID{}, AnyUUID{}, "aaaaaaaa-ae27-45c6-ab0f-c8cd9a48ebc2",
		AnyUUID{}, AnyTime{}).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(copyQueryRegex).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectPrepare(updateQueryRegex).WillReturnError(fmt.Errorf("Syntax error"))
	mock.ExpectRollback()
	mock.ExpectClose()

	req := httptest.NewRequest("PATCH", "/v2/respondents/be70e086-7bbc-461c-a565-5b454d748a71", bytes.NewBuffer(jsonOut))
	req.SetBasicAuth("admin", "secret")
	router.ServeHTTP(resp, req)

	var errResp models.Error
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'PATCH /respondents', ", err.Error())
	}

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Equal(t, "Error creating DB prepared statement: Syntax error", errResp.Error)
	assert.True(t, gock.IsDone())
}
