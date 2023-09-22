package test

import (
	// "bufio"

	"crypto/ecdsa"

	"flag"
	"log"
	"os"

	ipfsLog "github.com/ipfs/go-log/v2"
	utilKey "github.com/tinyverse-web3/mtv_go_utils/key"
)

const logName = "tvbase_test"

var testLog = ipfsLog.Logger(logName)

func parseCmdParams() (string, string, string) {
	help := flag.Bool("help", false, "Display help")
	srcSeed := flag.String("srcSeed", "", "src user pubkey")
	destSeed := flag.String("destSeed", "", "desc user pubkey")
	rootPath := flag.String("rootPath", "", "config file path")

	flag.Parse()

	if *help {
		testLog.Info("tinverse tvnode light")
		testLog.Info("Usage 1(default program run path): Run './tvnodelight -srcSeed softwarecheng@gmail.com' -destSeed softwarecheng@126.com")
		testLog.Info("Usage 2(special data root path): Run './tvnodelight -srcSeed softwarecheng@gmail.com' -destSeed softwarecheng@126.com, -rootPath ./light1")
		os.Exit(0)
	}

	if *srcSeed == "" {
		log.Fatal("Please provide seed for generate src user public key")
	}
	if *destSeed == "" {
		log.Fatal("Please provide seed for generate dest user public key")
	}

	return *srcSeed, *destSeed, *rootPath
}

func getKeyBySeed(seed string) (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	prikey, pubkey, err := utilKey.GenerateEcdsaKey(seed)
	if err != nil {
		return nil, nil, err
	}
	return prikey, pubkey, nil
}
