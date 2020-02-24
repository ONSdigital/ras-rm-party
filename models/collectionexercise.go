package models

type (
	// CollectionExercise represents the response from the Collection Exercise service's GET /collectionexercises/{id}
	// It does not represent the full response, just what we end up using
	CollectionExercise struct {
		ID       string `json:"id"`
		SurveyID string `json:"surveyId"`
	}
)
