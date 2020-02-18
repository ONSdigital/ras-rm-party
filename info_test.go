package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ONSdigital/ras-rm-party/models"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestInfo(t *testing.T) {
	setDefaults()
	setup()

	req := httptest.NewRequest("GET", "/v2/info/", nil)
	router.ServeHTTP(resp, req)
	body, _ := ioutil.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.Code)

	var infoResp models.Info
	err := json.Unmarshal(body, &infoResp)
	if err != nil {
		t.Fatal("Error decoding JSON response from 'GET /info', ", err.Error())
	}

	assert.Equal(t, viper.GetString("service_name"), infoResp.Name)
}
