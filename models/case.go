package models

type (
	caseGroup struct {
		ID                   string `json:"id"`
		CollectionExerciseID string `json:"collectionExerciseId"`
	}

	// Case represents the response from the Case service's GET /cases/{code}
	// It does not represent the full response, just what we end up using
	Case struct {
		ID         string    `json:"id"`
		BusinessID string    `json:"partyId"`
		CaseGroup  caseGroup `json:"caseGroup"`
	}
)
