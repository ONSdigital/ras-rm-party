package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/ONSdigital/ras-rm-party/models"
	"github.com/Unleash/unleash-client-go/v3"
	"github.com/julienschmidt/httprouter"
)

func getRespondents(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if !unleash.IsEnabled("party.api.get.respondents", unleash.WithFallback(false)) {
		w.WriteHeader(http.StatusMethodNotAllowed)
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

	// Check all params provided are valid
	for k := range queryParams {
		switch k {
		case
			"firstName", "lastName", "emailAddress", "telephone", "status", "businessId", "surveyId", "offset", "limit":
		default:
			w.WriteHeader(http.StatusBadRequest)
			errorString := models.Error{
				Error: "Invalid query parameter " + k,
			}
			json.NewEncoder(w).Encode(errorString)
		}
	}

	if db == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	_, err := db.Query("SELECT * from partysvc.respondent")
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		log.Println("Error querying DB:", err.Error())
		return
	}

	w.WriteHeader(http.StatusNotImplemented)
}
