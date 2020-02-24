package models

type (
	// CaseGroup represents information about a case's case group, including collection exercise information
	// It does not represent the full response, just what we end up using
	CaseGroup struct {
		ID                   string `json:"id"`
		CollectionExerciseID string `json:"collectionExerciseId"`
	}

	// Case represents the response from the Case service's GET /cases/{id}
	// It does not represent the full response, just what we end up using
	Case struct {
		ID         string    `json:"id"`
		BusinessID string    `json:"partyId"`
		CaseGroup  CaseGroup `json:"caseGroup"`
	}
)
