package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/ONSdigital/ras-rm-party/models"
	"github.com/Unleash/unleash-client-go/v3"
	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
	"github.com/lib/pq"
	"github.com/spf13/viper"
)

// Represents the data retrieved from other services about an enrolment
type newEnrolment struct {
	IAC      models.IAC
	Case     models.Case
	SurveyID string
}

func rowsToRespondentsModel(rows *sql.Rows) models.Respondents {
	respMap := make(map[string]*models.Respondent)
	respondents := models.Respondents{}
	for rows.Next() {
		respondent := models.Respondent{
			Attributes:   models.Attributes{},
			Associations: []models.Association{},
		}
		association := models.Association{Enrolments: []models.Enrolment{}}
		enrolment := models.Enrolment{}

		rows.Scan(
			&respondent.Attributes.ID,
			&respondent.Attributes.EmailAddress,
			&respondent.Attributes.FirstName,
			&respondent.Attributes.LastName,
			&respondent.Attributes.Telephone,
			&respondent.Status,
			&association.ID,
			&enrolment.SurveyID,
			&enrolment.EnrolmentStatus,
		)

		// If we already have this respondent in the rowset, it's a new association or enrolment
		if val, ok := respMap[respondent.Attributes.ID]; ok {
			found := false
			// If we already have this business association, it's a new enrolment for that association
			for idx := range val.Associations {
				if val.Associations[idx].ID == association.ID {
					found = true
					val.Associations[idx].Enrolments = append(val.Associations[idx].Enrolments, enrolment)
					break
				}
			}
			if !found {
				// Only add the enrolment if there actually is one
				if enrolment.EnrolmentStatus != "" && enrolment.SurveyID != "" {
					association.Enrolments = append(association.Enrolments, enrolment)
				}
				val.Associations = append(val.Associations, association)
			}
		} else {
			// Only add the enrolment if there actually is one
			if enrolment.EnrolmentStatus != "" && enrolment.SurveyID != "" {
				association.Enrolments = append(association.Enrolments, enrolment)
			}
			respondent.Associations = append(respondent.Associations, association)
			respMap[respondent.Attributes.ID] = &respondent
		}
	}

	for _, val := range respMap {
		respondents.Data = append(respondents.Data, *val)
	}

	return respondents
}

func checkRowsForBusinessIDs(rows *sql.Rows, enrolments map[string]*newEnrolment) (codeMissing string, ok bool) {
	var existingBusinesses []string
	if rows != nil {
		for rows.Next() {
			var id string
			rows.Scan(&id)
			existingBusinesses = append(existingBusinesses, id)
		}
	}

	for code, enrolment := range enrolments {
		found := false
		for _, id := range existingBusinesses {
			if enrolment.Case.BusinessID == id {
				found = true
				break
			}
		}
		if !found {
			return code, false
		}
	}

	return "", true
}

func stringArrayContains(targetArray []string, val string) bool {
	found := false
	for _, str := range targetArray {
		if str == val {
			found = true
			break
		}
	}
	return found
}

func getRespondents(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if !unleash.IsEnabled("party.api.get.respondents", unleash.WithFallback(false)) {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if db == nil {
		w.WriteHeader(http.StatusInternalServerError)
		errorString := models.Error{
			Error: "Database connection could not be found",
		}
		json.NewEncoder(w).Encode(errorString)
		return
	}

	queryParams := r.URL.Query()
	if len(queryParams) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		errorString := models.Error{
			Error: "No query parameters provided for search",
		}
		json.NewEncoder(w).Encode(errorString)
		return
	}

	// Check all params provided are valid and build a query string
	var sb strings.Builder
	sb.WriteString(" WHERE 1=1")
	for k := range queryParams {
		switch k {
		case "firstName":
			sb.WriteString(" AND r.first_name='")
			sb.WriteString(queryParams.Get("firstName"))
			sb.WriteString("'")
		case "lastName":
			sb.WriteString(" AND r.last_name='")
			sb.WriteString(queryParams.Get("lastName"))
			sb.WriteString("'")
		case "emailAddress":
			sb.WriteString(" AND r.email_address='")
			sb.WriteString(queryParams.Get("emailAddress"))
			sb.WriteString("'")
		case "telephone":
			sb.WriteString(" AND r.telephone='")
			sb.WriteString(queryParams.Get("telephone"))
			sb.WriteString("'")
		case "status":
			sb.WriteString(" AND r.status='")
			sb.WriteString(queryParams.Get("status"))
			sb.WriteString("'")
		case "businessId":
			sb.WriteString(" AND br.business_id='")
			sb.WriteString(queryParams.Get("businessId"))
			sb.WriteString("'")
		case "surveyId":
			sb.WriteString(" AND e.survey_id='")
			sb.WriteString(queryParams.Get("surveyId"))
			sb.WriteString("'")
		case "offset", "limit":
			// Fall through - we want to ensure these are at the end
		default:
			w.WriteHeader(http.StatusBadRequest)
			errorString := models.Error{
				Error: "Invalid query parameter " + k,
			}
			json.NewEncoder(w).Encode(errorString)
			return
		}
	}

	if queryParams.Get("offset") != "" {
		sb.WriteString(" OFFSET ")
		sb.WriteString(queryParams.Get("offset"))
	}

	if queryParams.Get("limit") != "" {
		sb.WriteString(" LIMIT ")
		sb.WriteString(queryParams.Get("limit"))
	}

	queryString := "SELECT r.id, r.email_address, r.first_name, r.last_name, r.telephone, r.status, br.business_id, e.status AS enrolment_status, e.survey_id " +
		"FROM partysvc.respondent r JOIN partysvc.business_respondent br ON r.id=br.respondent_id " +
		"JOIN partysvc.enrolment e ON br.business_id=e.business_id AND br.respondent_id=e.respondent_id" + sb.String()

	rows, err := db.Query(queryString)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		errorString := models.Error{
			Error: "Error querying DB: " + err.Error(),
		}
		json.NewEncoder(w).Encode(errorString)
		return
	}

	respondents := rowsToRespondentsModel(rows)

	if len(respondents.Data) == 0 {
		w.WriteHeader(http.StatusNotFound)
		errorString := models.Error{
			Error: "No respondents found",
		}
		json.NewEncoder(w).Encode(errorString)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(respondents)
}

func postRespondents(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if !unleash.IsEnabled("party.api.post.respondents", unleash.WithFallback(false)) {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if db == nil {
		w.WriteHeader(http.StatusInternalServerError)
		errorString := models.Error{
			Error: "Database connection could not be found",
		}
		json.NewEncoder(w).Encode(errorString)
		return
	}

	var postRequest models.PostRespondents
	err := json.NewDecoder(r.Body).Decode(&postRequest)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		errorString := models.Error{
			Error: "Invalid JSON",
		}
		json.NewEncoder(w).Encode(errorString)
		return
	}

	missingFields := []string{}
	if postRequest.Data.Attributes.EmailAddress == "" {
		missingFields = append(missingFields, "emailAddress")
	}
	if postRequest.Data.Attributes.FirstName == "" {
		missingFields = append(missingFields, "firstName")
	}
	if postRequest.Data.Attributes.LastName == "" {
		missingFields = append(missingFields, "lastName")
	}
	if postRequest.Data.Attributes.Telephone == "" {
		missingFields = append(missingFields, "telephone")
	}
	if len(postRequest.EnrolmentCodes) == 0 {
		missingFields = append(missingFields, "enrolmentCodes")
	}

	if len(missingFields) > 0 {
		w.WriteHeader(http.StatusBadRequest)
		errorString := models.Error{
			Error: "Missing required fields: " + strings.Join(missingFields, ", "),
		}
		json.NewEncoder(w).Encode(errorString)
		return
	}

	enrolments := map[string]*newEnrolment{}
	// Check enrolment codes
	for _, code := range postRequest.EnrolmentCodes {
		resp, err := http.Get(viper.GetString("iac_service") + "/iacs/" + code)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			errorString := models.Error{
				Error: "Couldn't communicate with IAC service: " + err.Error(),
			}
			json.NewEncoder(w).Encode(errorString)
			return
		}
		if resp.StatusCode == http.StatusNotFound {
			w.WriteHeader(http.StatusNotFound)
			errorString := models.Error{
				Error: "Enrolment code not found: " + code,
			}
			json.NewEncoder(w).Encode(errorString)
			return
		}
		iac := models.IAC{}
		json.NewDecoder(resp.Body).Decode(&iac)

		if !iac.Active {
			w.WriteHeader(http.StatusUnprocessableEntity)
			errorString := models.Error{
				Error: "Enrolment code inactive: " + code,
			}
			json.NewEncoder(w).Encode(errorString)
			return
		}
		enrolments[code] = &newEnrolment{IAC: iac}
	}

	// Check cases and collection exercises and build up the business check
	businessIDs := []string{}
	for code, enrolment := range enrolments {
		// Case service
		resp, err := http.Get(viper.GetString("case_service") + "/cases/" + enrolment.IAC.CaseID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			errorString := models.Error{
				Error: "Couldn't communicate with Case service: " + err.Error(),
			}
			json.NewEncoder(w).Encode(errorString)
			return
		}
		if resp.StatusCode == http.StatusNotFound {
			w.WriteHeader(http.StatusNotFound)
			errorString := models.Error{
				Error: "Case not found for enrolment code: " + code,
			}
			json.NewEncoder(w).Encode(errorString)
			return
		}

		json.NewDecoder(resp.Body).Decode(&enrolment.Case)
		if !stringArrayContains(businessIDs, enrolment.Case.BusinessID) {
			businessIDs = append(businessIDs, enrolment.Case.BusinessID)
		}

		// Collection Exercise service
		resp, err = http.Get(viper.GetString("collection_exercise_service") + "/collectionexercises/" + enrolment.Case.CaseGroup.CollectionExerciseID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			errorString := models.Error{
				Error: "Couldn't communicate with Collection Exercise service: " + err.Error(),
			}
			json.NewEncoder(w).Encode(errorString)
			return
		}
		if resp.StatusCode == http.StatusNotFound {
			w.WriteHeader(http.StatusNotFound)
			errorString := models.Error{
				Error: "Collection Exercise not found for enrolment code: " + code,
			}
			json.NewEncoder(w).Encode(errorString)
			return
		}

		collectionExercise := models.CollectionExercise{}
		json.NewDecoder(resp.Body).Decode(&collectionExercise)
		enrolment.SurveyID = collectionExercise.SurveyID
	}

	// Ensure that all the businesses we want to associate with exist
	businessQuery, err := db.Prepare("SELECT party_uuid FROM partysvc.business WHERE party_uuid=ANY($1)")
	defer businessQuery.Close()
	rows, err := businessQuery.Query(pq.Array(businessIDs))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		errorString := models.Error{
			Error: "Error querying DB: " + err.Error(),
		}
		json.NewEncoder(w).Encode(errorString)
		return
	}

	if missingCode, ok := checkRowsForBusinessIDs(rows, enrolments); !ok {
		// Won't be able to associate with a business we can't find
		w.WriteHeader(http.StatusUnprocessableEntity)
		errorString := models.Error{
			Error: "Can't associate with the business for enrolment code: " + missingCode,
		}
		json.NewEncoder(w).Encode(errorString)
		return
	}

	tx, err := db.Begin()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		errorString := models.Error{
			Error: "Error creating DB transaction: " + err.Error(),
		}
		json.NewEncoder(w).Encode(errorString)
		return
	}

	insertRespondent, err := tx.Prepare("INSERT INTO partysvc.respondent (id, status, email_address, first_name, last_name, telephone, created_on) VALUES ($1,$2,$3,$4,$5,$6,$7)")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		errorString := models.Error{
			Error: "Error creating DB prepared statement: " + err.Error(),
		}
		json.NewEncoder(w).Encode(errorString)
		return
	}

	respondentID := postRequest.Data.Attributes.ID
	if respondentID == "" {
		respondentID = uuid.New().String()
	}

	defer insertRespondent.Close()
	_, err = insertRespondent.Exec(respondentID, "CREATED", postRequest.Data.Attributes.EmailAddress, postRequest.Data.Attributes.FirstName,
		postRequest.Data.Attributes.LastName, postRequest.Data.Attributes.Telephone, time.Now())
	if err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		errorString := models.Error{
			Error: "Can't create a respondent with ID " + respondentID + ": " + err.Error(),
		}
		json.NewEncoder(w).Encode(errorString)
		tx.Rollback()
		return
	}

	insertBusinessRespondent, err := tx.Prepare(pq.CopyIn("partysvc.respondent", "business_id", "respondent_id", "status", "effective_from", "created_on"))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		errorString := models.Error{
			Error: "Error creating DB prepared statement: " + err.Error(),
		}
		json.NewEncoder(w).Encode(errorString)
		return
	}
	defer insertBusinessRespondent.Close()
	for _, business := range businessIDs {
		_, err = insertBusinessRespondent.Exec(business, respondentID, "ACTIVE", time.Now(), time.Now())
		if err != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			errorString := models.Error{
				Error: "Can't create a business/respondent link with respondent ID " + respondentID + " and business ID " + business + ": " + err.Error(),
			}
			json.NewEncoder(w).Encode(errorString)
			tx.Rollback()
			return
		}
	}
	_, err = insertBusinessRespondent.Exec()
	if err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		errorString := models.Error{
			Error: "Can't commit business/respondent links with respondent ID " + respondentID + ": " + err.Error(),
		}
		json.NewEncoder(w).Encode(errorString)
		tx.Rollback()
		return
	}

	insertPendingEnrolment, err := tx.Prepare(pq.CopyIn("partysvc.pending_enrolment", "case_id", "respondent_id", "business_id", "survey_id", "created_on"))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		errorString := models.Error{
			Error: "Error creating DB prepared statement: " + err.Error(),
		}
		json.NewEncoder(w).Encode(errorString)
		return
	}
	defer insertPendingEnrolment.Close()

	insertEnrolment, err := tx.Prepare(pq.CopyIn("partysvc.enrolment", "respondent_id", "business_id", "survey_id", "status", "created_on"))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		errorString := models.Error{
			Error: "Error creating DB prepared statement: " + err.Error(),
		}
		json.NewEncoder(w).Encode(errorString)
		return
	}
	defer insertEnrolment.Close()

	newAssociations := []models.Association{}
	for _, enrolment := range enrolments {
		_, err := insertPendingEnrolment.Exec(enrolment.Case.ID, respondentID, enrolment.Case.BusinessID, enrolment.SurveyID, time.Now())
		if err != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			errorString := models.Error{
				Error: "Can't create a Pending Enrolment with respondent ID " + respondentID + " and business ID " + enrolment.Case.BusinessID + ": " + err.Error(),
			}
			json.NewEncoder(w).Encode(errorString)
			tx.Rollback()
			return
		}

		_, err = insertEnrolment.Exec(respondentID, enrolment.Case.BusinessID, enrolment.SurveyID, "PENDING", time.Now())
		if err != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			errorString := models.Error{
				Error: "Can't create an Enrolment with respondent ID " + respondentID + " and business ID " + enrolment.Case.BusinessID + ": " + err.Error(),
			}
			json.NewEncoder(w).Encode(errorString)
			tx.Rollback()
			return
		}

		found := false
		newEnrolment := models.Enrolment{
			EnrolmentStatus: "PENDING",
			SurveyID:        enrolment.SurveyID,
		}
		for idx := range newAssociations {
			if newAssociations[idx].ID == enrolment.Case.BusinessID {
				found = true
				newAssociations[idx].Enrolments = append(newAssociations[idx].Enrolments, newEnrolment)
			}
		}
		if !found {
			newAssociations = append(newAssociations, models.Association{
				ID: enrolment.Case.BusinessID,
				Enrolments: []models.Enrolment{
					newEnrolment,
				},
			})
		}
	}

	_, err = insertPendingEnrolment.Exec()
	if err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		errorString := models.Error{
			Error: "Can't commit pending enrolments with respondent ID " + respondentID + ": " + err.Error(),
		}
		json.NewEncoder(w).Encode(errorString)
		tx.Rollback()
		return
	}
	_, err = insertEnrolment.Exec()
	if err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		errorString := models.Error{
			Error: "Can't commit enrolments with respondent ID " + respondentID + ": " + err.Error(),
		}
		json.NewEncoder(w).Encode(errorString)
		tx.Rollback()
		return
	}

	err = tx.Commit()
	if err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		errorString := models.Error{
			Error: "Can't commit database transaction for respondent ID " + respondentID + ": " + err.Error(),
		}
		json.NewEncoder(w).Encode(errorString)
		tx.Rollback()
		return
	}

	// Deactivate the enrolment codes
	for _, code := range postRequest.EnrolmentCodes {
		// IAC service
		body := bytes.NewBuffer([]byte(`{"updatedBy": "Party Service"}`))
		req, _ := http.NewRequest(http.MethodPut, viper.GetString("iac_service")+"/"+code, body)
		resp, err := http.DefaultClient.Do(req)
		// It's fine if this fails - log the error and move on. We should still give a 200 OK response
		if err != nil {
			log.Println("Error deactivating enrolment code " + code + ": " + err.Error())
			continue
		}
		if resp.StatusCode != http.StatusOK {
			log.Println("Error deactivating enrolment code " + code + ": Received status code " + strconv.Itoa(resp.StatusCode) + " from IAC service")
			continue
		}
	}

	response := models.Respondents{
		Data: []models.Respondent{
			models.Respondent{
				Attributes: models.Attributes{
					ID:           respondentID,
					EmailAddress: postRequest.Data.Attributes.EmailAddress,
					FirstName:    postRequest.Data.Attributes.FirstName,
					LastName:     postRequest.Data.Attributes.LastName,
					Telephone:    postRequest.Data.Attributes.Telephone,
				},
				Status:       "ACTIVE",
				Associations: newAssociations,
			}}}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
	return
}

func deleteRespondents(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	if !unleash.IsEnabled("party.api.delete.respondents.id", unleash.WithFallback(false)) {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	respondentUUID, err := uuid.Parse(p.ByName("id"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		errorString := models.Error{
			Error: "Not a valid ID: " + p.ByName("id"),
		}
		json.NewEncoder(w).Encode(errorString)
		return
	}

	if db == nil {
		w.WriteHeader(http.StatusInternalServerError)
		errorString := models.Error{
			Error: "Database connection could not be found",
		}
		json.NewEncoder(w).Encode(errorString)
		return
	}

	var respondentID string
	err = db.QueryRow("SELECT id FROM partysvc.respondents WHERE id=$1", respondentUUID.String()).Scan(&respondentID)
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
			errorString := models.Error{
				Error: "Respondent does not exist",
			}
			json.NewEncoder(w).Encode(errorString)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			errorString := models.Error{
				Error: "Error querying DB: " + err.Error(),
			}
			json.NewEncoder(w).Encode(errorString)
		}
		return
	}

	tx, err := db.Begin()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		errorString := models.Error{
			Error: "Error creating DB transaction: " + err.Error(),
		}
		json.NewEncoder(w).Encode(errorString)
		return
	}

	_, err = tx.Exec("DELETE FROM partysvc.enrolment WHERE respondent_id=$1", respondentID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		errorString := models.Error{
			Error: "Error deleting enrolments for respondent ID " + respondentID + ": " + err.Error(),
		}
		json.NewEncoder(w).Encode(errorString)
		tx.Rollback()
		return
	}

	_, err = tx.Exec("DELETE FROM partysvc.business_respondent WHERE respondent_id=$1", respondentID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		errorString := models.Error{
			Error: "Error deleting business respondent for respondent ID " + respondentID + ": " + err.Error(),
		}
		json.NewEncoder(w).Encode(errorString)
		tx.Rollback()
		return
	}

	_, err = tx.Exec("DELETE FROM partysvc.pending_enrolment WHERE respondent_id=$1", respondentID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		errorString := models.Error{
			Error: "Error deleting pending enrolments for respondent ID " + respondentID + ": " + err.Error(),
		}
		json.NewEncoder(w).Encode(errorString)
		tx.Rollback()
		return
	}

	_, err = tx.Exec("DELETE FROM partysvc.respondent WHERE id=$1", respondentID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		errorString := models.Error{
			Error: "Error deleting respondent record for respondent ID " + respondentID + ": " + err.Error(),
		}
		json.NewEncoder(w).Encode(errorString)
		tx.Rollback()
		return
	}

	err = tx.Commit()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		errorString := models.Error{
			Error: "Can't commit transaction for respondent ID " + respondentID + ": " + err.Error(),
		}
		json.NewEncoder(w).Encode(errorString)
		tx.Rollback()
		return
	}

	log.Println("Successfully deleted respondent " + respondentID)
	w.WriteHeader(http.StatusNoContent)
	return
}

func getRespondentsByID(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	if !unleash.IsEnabled("party.api.get.respondents.id", unleash.WithFallback(false)) {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	respondentID, err := uuid.Parse(p.ByName("id"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		errorString := models.Error{
			Error: "Not a valid ID: " + p.ByName("id"),
		}
		json.NewEncoder(w).Encode(errorString)
		return
	}

	if db == nil {
		w.WriteHeader(http.StatusInternalServerError)
		errorString := models.Error{
			Error: "Database connection could not be found",
		}
		json.NewEncoder(w).Encode(errorString)
		return
	}

	rows, err := db.Query("SELECT r.id, r.email_address, r.first_name, r.last_name, r.telephone, r.status, br.business_id, e.status AS enrolment_status, e.survey_id "+
		"FROM partysvc.respondent r JOIN partysvc.business_respondent br ON r.id=br.respondent_id "+
		"JOIN partysvc.enrolment e ON br.business_id=e.business_id AND br.respondent_id=e.respondent_id "+
		"WHERE r.id=$1", respondentID.String())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		errorString := models.Error{
			Error: "Error querying DB: " + err.Error(),
		}
		json.NewEncoder(w).Encode(errorString)
		return
	}

	respondents := rowsToRespondentsModel(rows)

	if len(respondents.Data) == 0 {
		w.WriteHeader(http.StatusNotFound)
		errorString := models.Error{
			Error: "No respondent found for ID " + respondentID.String(),
		}
		json.NewEncoder(w).Encode(errorString)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(respondents)
}

func patchRespondentsByID(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	if !unleash.IsEnabled("party.api.patch.respondents.id", unleash.WithFallback(false)) {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	respondentUUID, err := uuid.Parse(p.ByName("id"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		errorString := models.Error{
			Error: "Not a valid ID: " + p.ByName("id"),
		}
		json.NewEncoder(w).Encode(errorString)
		return
	}

	var postRequest models.PostRespondents
	err = json.NewDecoder(r.Body).Decode(&postRequest)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		errorString := models.Error{
			Error: "Invalid JSON",
		}
		json.NewEncoder(w).Encode(errorString)
		return
	}

	if postRequest.Data.Attributes.ID != "" && postRequest.Data.Attributes.ID != respondentUUID.String() {
		w.WriteHeader(http.StatusBadRequest)
		errorString := models.Error{
			Error: "ID must not be changed",
		}
		json.NewEncoder(w).Encode(errorString)
		return
	}

	if db == nil {
		w.WriteHeader(http.StatusInternalServerError)
		errorString := models.Error{
			Error: "Database connection could not be found",
		}
		json.NewEncoder(w).Encode(errorString)
		return
	}

	_, err = db.Begin()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		errorString := models.Error{
			Error: "Error creating DB transaction: " + err.Error(),
		}
		json.NewEncoder(w).Encode(errorString)
		return
	}

	var respondentID string
	var emailAddress string
	err = db.QueryRow("SELECT id, email_address FROM partysvc.respondents WHERE id=$1", respondentUUID.String()).Scan(&respondentID, &emailAddress)
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
			errorString := models.Error{
				Error: "Respondent does not exist",
			}
			json.NewEncoder(w).Encode(errorString)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			errorString := models.Error{
				Error: "Error querying DB: " + err.Error(),
			}
			json.NewEncoder(w).Encode(errorString)
		}
		return
	}

	if !reflect.DeepEqual(models.Respondent{}, postRequest.Data) {
		if emailAddress != postRequest.Data.Attributes.EmailAddress {
			var count int
			err = db.QueryRow("SELECT COUNT(*) FROM partysvc.respondents WHERE email_address=$1", postRequest.Data.Attributes.EmailAddress).Scan(&count)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				errorString := models.Error{
					Error: "Error querying DB: " + err.Error(),
				}
				json.NewEncoder(w).Encode(errorString)
			}
			if count > 0 {
				w.WriteHeader(http.StatusConflict)
				errorString := models.Error{
					Error: "New email address already in use",
				}
				json.NewEncoder(w).Encode(errorString)
			}
		}
	}

	w.WriteHeader(http.StatusOK)
}
