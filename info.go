package main

import (
	"encoding/json"
	"net/http"

	"github.com/ONSdigital/ras-rm-party/models"
	"github.com/julienschmidt/httprouter"
	"github.com/spf13/viper"
)

func getInfo(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	info := models.Info{
		Name:    "Hello world",
		Version: viper.GetString("app_version"),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}
