package main

import "github.com/spf13/viper"

func setDefaults() {
	viper.SetDefault("unleash_uri", "http://localhost:4242/api")
	viper.SetDefault("service_name", "ras-rm-party")
	viper.SetDefault("port", "8059")
	viper.SetDefault("app_version", "unknown")
}
