package main

import (
	"flag"
	"log"

	"github.com/insomniacslk/slackbot/pkg/bot"
	_ "github.com/insomniacslk/slackbot/plugins/oncall"
	_ "github.com/insomniacslk/slackbot/plugins/pinger"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var (
	flagConfig = flag.String("c", "", "Configuration file")
)

func main() {
	flag.Parse()

	viper.SetConfigFile(*flagConfig)
	viper.SetConfigType("yaml")
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// TODO create new config file
			logrus.Fatalf("Config file not found")
		} else {
			logrus.Fatalf("Failed to read config file: %v", err)
		}
	}
	var config bot.Config
	if err := viper.Unmarshal(&config); err != nil {
		logrus.Fatalf("Failed to unmarshal config: %v", err)
	}
	if err := config.Validate(); err != nil {
		logrus.Fatalf("Invalid config: %v", err)
	}

	b := bot.New(&config)
	if err := b.Start(); err != nil {
		log.Fatal(err)
	}
}
