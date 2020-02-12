package main

import "github.com/spf13/viper"

func setDefaults() {
	viper.SetDefault("unleash_path", "http://localhost:4242/api")
	viper.SetDefault("service_name", "ras-rm-party")
	viper.SetDefault("listen_port", "8081")
}
