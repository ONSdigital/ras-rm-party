package models

type (
	// Enrolment respresents a single survey enrolment for the respondent
	Enrolment struct {
		EnrolmentStatus string `json:"enrolmentStatus"`
		SurveyID        string `json:"surveyId"`
	}

	// Association represents a single business associated to a respondent
	Association struct {
		Enrolments    []Enrolment `json:"enrolments"`
		Name          string      `json:"name"`
		ID            string      `json:"id"`
		SampleUnitRef string      `json:"sampleUnitRef"`
	}

	// Attributes represents the attributes of a single respondent
	Attributes struct {
		EmailAddress string `json:"emailAddress"`
		ID           string `json:"id"`
		FirstName    string `json:"firstName"`
		LastName     string `json:"lastName"`
		Telephone    string `json:"telephone"`
	}

	// Respondent respresents a single respondent
	Respondent struct {
		Attributes   Attributes    `json:"attributes"`
		Status       string        `json:"status"`
		Associations []Association `json:"associations"`
	}

	// Respondents represents the response from all non-DELETE /respondents endpoints
	Respondents struct {
		Data []Respondent `json:"data"`
	}

	// PostRespondents represents the expected format of a POST /respondents Request-Body
	PostRespondents struct {
		Data           Respondent `json:"data"`
		EnrolmentCodes []string   `json:"enrolmentCodes"`
	}
)
