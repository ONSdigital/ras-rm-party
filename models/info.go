package models

type (
	// Info represents the response from 'GET /info'
	Info struct {
		Name    string
		Version string
	}
)
