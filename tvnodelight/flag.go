package main

import (
	"flag"
	"log"
	"os"
)

func parseCmdParams() (string, string, string, string) {
	help := flag.Bool("help", false, "display help")
	srcseed := flag.String("src", "", "src user pubkey")
	destseed := flag.String("dest", "", "desc user pubkey")
	channelseed := flag.String("channel", "", "channel pubkey")
	path := flag.String("path", "", "all data path")

	flag.Parse()

	if *help {
		logger.Info("tinverse tvnodelight")
		logger.Info("Usage Run './tvnodelight -src softwarecheng@gmail.com -dest softwarecheng@126.com -channel softwarecheng@126.com -path .'")
		os.Exit(0)
	}

	if *srcseed == "" {
		log.Fatal("Please provide seed for generate src user seed for public key")
	}
	if *destseed == "" {
		log.Fatal("Please provide seed for generate dest user seed for public key")
	}

	if *channelseed == "" {
		log.Fatal("Please provide seed for generate channel seed for public key")
	}

	return *srcseed, *destseed, *channelseed, *path
}
