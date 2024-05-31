package main

import (
	"flag"
	"log"

	"github.com/insomniacslk/slack-bots/pkg/bot"
	_ "github.com/insomniacslk/slack-bots/plugins/oncall"
)

var (
	flagConfig = flag.String("c", "", "Configuration file")
)

func main() {
	flag.Parse()
	config, err := bot.ReadConfig(*flagConfig)
	if err != nil {
		log.Fatalf("Configuration file error: %v", err)
	}
	b := bot.New(config)
	if err := b.Start(); err != nil {
		log.Fatal(err)
	}
}
