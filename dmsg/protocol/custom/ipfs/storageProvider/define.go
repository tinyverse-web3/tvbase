package storageprovider

import (
	"time"

	ipfsLog "github.com/ipfs/go-log/v2"
)

const (
	NftProvider  = "nft"
	Web3Provider = "web3"

	Web3PostURL    = "https://api.web3.storage/upload"
	Web3CarPostURL = "https://api.web3.storage/car/upload"
	NftPostURL     = "https://api.nft.storage/upload"

	loggerName = "dmsg.protocol.custom.ipfs.storageProvider"
)

var (
	NftApiKey  = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJkaWQ6ZXRocjoweDgxOTgwNzg4Y2UxQjY3MDQyM2Y1NzAyMDQ2OWM0MzI3YzNBNzU5YzciLCJpc3MiOiJuZnQtc3RvcmFnZSIsImlhdCI6MTY5MDUyODU1MjcxMywibmFtZSI6InRlc3QxIn0.vslsn8tAWUtZ0BZjcxhyMrcuufwfZ7fTMpF_DrojF4c"
	Web3ApiKey = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJkaWQ6ZXRocjoweDAyYzZEYkJBMTQyOTA1MzliZjgwNkEzRkNDRDgzMDFmNWNjNTQ2ZDIiLCJpc3MiOiJ3ZWIzLXN0b3JhZ2UiLCJpYXQiOjE2OTA2ODkxMjg3NDUsIm5hbWUiOiJ0ZXN0In0.nhArwLJYjFwTiW1-SSRPyrCCczyYQ4T2PAHcShFZXqg"

	UploadInterval = 3 * time.Second
	UploadTimeout  = 30 * time.Second

	logger = ipfsLog.Logger(loggerName)
)
