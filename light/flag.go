package light

import (
	"flag"

	"github.com/tinyverse-web3/tvbase/common/config"
)

var IsTestEnv = false
var nodeMode config.NodeMode
var pkseed *string

func ParseCmdParams() (string, string, string, string) {
	src := flag.String("src", "", "Set src user pubkey")
	dest := flag.String("dest", "", "Set desc user pubkey")
	mode := flag.String("mode", "light", "Initialize tvnode mode for service mode or light mode.")
	channel := flag.String("channel", "", "Set channel pubkey")
	path := flag.String("path", "", "Path to configuration file and data file to use.")
	test := flag.Bool("test", false, "Operate in test environment.")
	pkseed = flag.String("pkseed", "", "Seed for generating peer private key in test environment.")
	flag.Parse()

	if *mode == "service" {
		nodeMode = config.ServiceMode
	}

	if *test {
		IsTestEnv = *test
	}

	if *src == "" {
		Logger.Fatal("Please provide seed for generate src user seed for public key")
	}
	if *dest == "" {
		Logger.Fatal("Please provide seed for generate dest user seed for public key")
	}

	if *channel == "" {
		Logger.Fatal("Please provide seed for generate channel seed for public key")
	}

	return *src, *dest, *channel, *path
}
