package models

type (
	// Info represents the response from 'GET /info'
	Info struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
)
