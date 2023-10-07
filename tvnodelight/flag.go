package main

import (
	"flag"
	"os"
)

var isTestEnv = false

func parseCmdParams() (string, string, string, string) {
	help := flag.Bool("help", false, "Show help.")
	src := flag.String("src", "", "Set src user pubkey")
	dest := flag.String("dest", "", "Set desc user pubkey")
	channel := flag.String("channel", "", "Set channel pubkey")
	path := flag.String("path", "", "Path to configuration file and data file to use.")
	test := flag.Bool("test", false, "Operate in test environment.")
	flag.Parse()

	if *help {
		logger.Info("tinverse tvnodelight")
		logger.Info("Usage Run './tvnodelight -src softwarecheng@gmail.com -dest softwarecheng@126.com -channel softwarecheng@126.com -path .'")
		os.Exit(0)
	}

	if *test {
		isTestEnv = *test
	}

	if *src == "" {
		logger.Fatal("Please provide seed for generate src user seed for public key")
	}
	if *dest == "" {
		logger.Fatal("Please provide seed for generate dest user seed for public key")
	}

	if *channel == "" {
		logger.Fatal("Please provide seed for generate channel seed for public key")
	}

	return *src, *dest, *channel, *path
}
