package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ONSdigital/ras-rm-party/models"
	"github.com/Unleash/unleash-client-go/v3"
	"github.com/julienschmidt/httprouter"
)

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
			sb.WriteString(" AND e.business_id='")
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

	queryString := "SELECT * from partysvc.respondent r JOIN partysvc.enrolment e ON r.id=e.respondent_id" + sb.String()

	if _, err := db.Query(queryString); err != nil {
		w.WriteHeader(http.StatusNotFound)
		errorString := models.Error{
			Error: "Error querying DB: " + err.Error(),
		}
		json.NewEncoder(w).Encode(errorString)
		return
	}

	w.WriteHeader(http.StatusNotImplemented)
}
