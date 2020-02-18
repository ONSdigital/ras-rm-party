package main

import (
	"encoding/json"
	"net/http"

	"github.com/ONSdigital/ras-rm-party/models"
	"github.com/julienschmidt/httprouter"
	"github.com/spf13/viper"
)

func info(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	info := models.Info{
		Name:    viper.GetString("service_name"),
		Version: viper.GetString("app_version"),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}
