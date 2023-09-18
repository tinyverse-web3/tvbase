package storageprovider

import (
	"time"
)

// https://nft.storage/api-docs/

type Nft struct {
	CommonProvider
	postUrl string
}

var nft *Nft

func GetNft() *Nft {
	return nft
}

func CreateNft(apikey string,
	intervalTime time.Duration,
	ratio int64,
	uploadMaxSize int,
	authHeaeder string,
	authPrefix string,
) *Nft {
	nft = &Nft{
		postUrl: NftPostURL,
	}
	nft.Init(apikey, intervalTime, ratio, uploadMaxSize, authHeaeder, authPrefix)
	return nft
}
