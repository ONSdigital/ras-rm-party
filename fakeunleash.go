package main

// Code by Jason Barron (@jrbarron) in the Unleash Slack server - the unleash-go client doesn't currently have client or server stubs yet.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"
)

type parameterMap map[string]interface{}

type featureResponse struct {
	Version  int       `json:"version"`
	Features []feature `json:"features"`
}

type feature struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Enabled     bool         `json:"enabled"`
	Strategies  []strategy   `json:"strategies"`
	CreatedAt   time.Time    `json:"createdAt"`
	Strategy    string       `json:"strategy"`
	Parameters  parameterMap `json:"parameters"`
}

type strategy struct {
	ID          int          `json:"id"`
	Name        string       `json:"name"`
	Constraints []string     `json:"constraints"`
	Parameters  parameterMap `json:"parameters"`
}

type fakeUnleashServer struct {
	sync.RWMutex
	srv      *httptest.Server
	features map[string]bool
}

func (f *fakeUnleashServer) url() string {
	return f.srv.URL
}

func (f *fakeUnleashServer) Enable(feature string) {
	f.setEnabled(feature, true)
}

func (f *fakeUnleashServer) Disable(feature string) {
	f.setEnabled(feature, false)
}

func (f *fakeUnleashServer) setEnabled(feature string, enabled bool) {
	f.Lock()
	wasEnabled := f.features[feature]
	if enabled != wasEnabled {
		f.features[feature] = enabled
	}
	f.Unlock()
}

func (f *fakeUnleashServer) IsEnabled(feature string) bool {
	f.RLock()
	enabled := f.features[feature]
	f.RUnlock()
	return enabled
}

func (f *fakeUnleashServer) setAll(enabled bool) {
	for k := range f.features {
		f.setEnabled(k, enabled)
	}
}

func (f *fakeUnleashServer) EnableAll() {
	f.setAll(true)
}

func (f *fakeUnleashServer) DisableAll() {
	f.setAll(false)
}

func (f *fakeUnleashServer) handler(w http.ResponseWriter, req *http.Request) {
	switch req.Method + " " + req.URL.Path {
	case "GET /client/features":

		features := []feature{}
		for k, v := range f.features {
			features = append(features, feature{
				Name:    k,
				Enabled: v,
				Strategies: []strategy{
					{
						ID:   0,
						Name: "default",
					},
				},
				CreatedAt: time.Time{},
			})
		}

		res := featureResponse{
			Version:  2,
			Features: features,
		}
		dec := json.NewEncoder(w)
		if err := dec.Encode(res); err != nil {
			println(err.Error())
		}
	case "POST /client/register":
		fallthrough
	case "POST /client/metrics":
		w.WriteHeader(200)
	default:
		w.Write([]byte("Unknown route"))
		w.WriteHeader(500)
	}
}

func newFakeUnleash() *fakeUnleashServer {
	faker := &fakeUnleashServer{
		features: map[string]bool{},
	}
	faker.srv = httptest.NewServer(http.HandlerFunc(faker.handler))
	return faker
}
