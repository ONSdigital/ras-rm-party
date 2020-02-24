package main

import "github.com/spf13/viper"

func setDefaults() {
	viper.SetDefault("unleash_uri", "http://localhost:4242/api")
	viper.SetDefault("service_name", "ras-rm-party")
	viper.SetDefault("port", "8059")
	viper.SetDefault("app_version", "unknown")
	viper.SetDefault("database_uri", "postgres://postgres:postgres@localhost:6432/postgres?sslmode=disable")
	viper.SetDefault("security_user_name", "admin")
	viper.SetDefault("security_user_password", "secret")
	viper.SetDefault("ras_iac_service_host", "http://localhost")
	viper.SetDefault("ras_iac_service_port", "8121")
	viper.SetDefault("iac_service", viper.GetString("ras_iac_service_host")+":"+viper.GetString("ras_iac_service_port"))
	viper.SetDefault("ras_case_service_host", "http://localhost")
	viper.SetDefault("ras_case_service_port", "8171")
	viper.SetDefault("case_service", viper.GetString("ras_case_service_host")+":"+viper.GetString("ras_case_service_port"))
}
