package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ONSdigital/ras-rm-party/models"
	"github.com/Unleash/unleash-client-go/v3"
	"github.com/julienschmidt/httprouter"
	"github.com/spf13/viper"
)

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

func getRespondents(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if !unleash.IsEnabled("party.api.get.respondents", unleash.WithFallback(false)) {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if db == nil {
		w.WriteHeader(http.StatusNotFound)
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
		"FROM partysvc.respondent JOIN partysvc.business_respondent br ON r.id=br.respondent_id " +
		"JOIN partysvc.enrolment e ON br.business_id=e.business_id AND br.respondent_id=e.respondent_id" + sb.String()

	rows, err := db.Query(queryString)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
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
		w.WriteHeader(http.StatusNotFound)
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

	enrolmentCodes := []models.IAC{}
	// Check enrolment codes
	for _, code := range postRequest.EnrolmentCodes {
		resp, err := http.Get(viper.GetString("iac_service") + "/iacs/" + code)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
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
		enrolmentCodes = append(enrolmentCodes, iac)
	}

	queryString := "INSERT INTO respondent VALUES (1, 1)"
	// TODO: add error handling
	tx, err := db.Begin()
	_, err = tx.Exec(queryString)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		errorString := models.Error{
			Error: "Error querying DB: " + err.Error(),
		}
		json.NewEncoder(w).Encode(errorString)
		tx.Rollback()
		return
	}

	// TODO: add error handling
	tx.Commit()

	w.WriteHeader(http.StatusCreated)
	return
}
