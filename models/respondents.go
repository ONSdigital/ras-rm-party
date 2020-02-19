package models

type (
	enrolment struct {
		EnrolmentStatus string `json:"enrolmentStatus"`
		SurveyID        string `json:"surveyId"`
	}

	association struct {
		Enrolments    []enrolment `json:"enrolments"`
		Name          string      `json:"name"`
		ID            string      `json:"id"`
		SampleUnitRef string      `json:"sampleUnitRef"`
	}

	attributes struct {
		EmailAddress string `json:"emailAddress"`
		ID           string `json:"id"`
		FirstName    string `json:"firstName"`
		LastName     string `json:"lastName"`
		Telephone    string `json:"telephone"`
	}

	respondent struct {
		Attributes   attributes    `json:"attributes"`
		Status       string        `json:"status"`
		Associations []association `json:"associations"`
	}

	// Respondents represents the response from all non-DELETE /respondents endpoints`
	Respondents struct {
		Data []respondent `json:"data"`
	}
)
