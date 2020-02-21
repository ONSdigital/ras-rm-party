package models

type (
	// IAC represents the response from the IAC service's GET /iacs/{code}
	IAC struct {
		IAC    string `json:"iac"`
		Active bool `json:"active"`
		LastUsed string `json:"lastUsedTimeDate"`
		CaseID string `json:"caseId"`
		QuestionSet string `json:"questionSet"`
	}
)