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
}
