package storageprovider

import "time"

// https://web3.storage/docs/reference/http-api/
// the same API key exceeds 30 request within 10 seconds, the rate limit will be triggered
type Web3 struct {
	CommonProvider
	postUrl string
}

var web3 *Web3

func GetWeb3() *Web3 {
	return web3
}

func CreateWeb3(
	apikey string,
	intervalTime time.Duration,
	ratio int64,
	uploadMaxSize int,
	authHeaeder string,
	authPrefix string,
) *Web3 {
	web3 = &Web3{
		postUrl: Web3PostURL,
	}
	web3.Init(apikey, intervalTime, ratio, uploadMaxSize, authHeaeder, authPrefix)
	return web3
}
