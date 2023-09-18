package storageprovider

import (
	"io"
	"net/http"
	"time"

	"github.com/juju/ratelimit"
	tvbaseIpfs "github.com/tinyverse-web3/tvbase/common/ipfs"
)

// https://nft.storage/api-docs/
type CommonProvider struct {
	apikey          string
	uploadMaxSize   int
	authHeader      string
	authPrefix      string
	ratelimitBucket *ratelimit.Bucket
}

func (p *CommonProvider) Init(
	apikey string,
	intervalTime time.Duration,
	ratio int64,
	uploadMaxSize int,
	authHeaeder string,
	authPrefix string,
) {
	p.apikey = apikey
	p.uploadMaxSize = uploadMaxSize
	p.authHeader = authHeaeder
	p.authPrefix = authPrefix
	p.ratelimitBucket = ratelimit.NewBucket(intervalTime, ratio)
}

func (p *CommonProvider) Upload(cid string, timeout time.Duration, PostUrl string) ([]byte, error) {
	logger.Debugf("CommonProvider->Updoad begin: cid: %s", cid)

	p.ratelimitBucket.Wait(1)

	sh := tvbaseIpfs.GetIpfsShellProxy()
	reader, err := sh.Cat(cid)
	if err != nil {
		logger.Errorf("CommonProvider->Updoad: sh.Cat error: %v", err)
		return nil, err
	}
	defer reader.Close()

	req, err := http.NewRequest("POST", PostUrl, reader)
	if err != nil {
		logger.Errorf("CommonProvider->Updoad: http.NewRequest error: %v", err)
		return nil, nil
	}

	// authheader := p.authHeader
	// value := "Bearer " + p.authPrefix + p.apikey
	req.Header.Set("Authorization", "Bearer "+p.apikey)

	_, size, err := sh.BlockStat(cid)
	if err != nil {
		logger.Errorf("CommonProvider->Updoad: sh.BlockStat error: %v", err)
		return nil, err
	}

	if size >= p.uploadMaxSize { // max size 100MB limit, big size file need to split more car format content
		logger.Errorf("CommonProvider->Updoad: file too large, uploadMaxSize: %d byte, factSize: %d byte", p.uploadMaxSize, size)
		return nil, err
	}

	client := &http.Client{
		Timeout: timeout,
	}
	resp, err := client.Do(req)
	if err != nil {
		logger.Errorf("CommonProvider->Updoad: client.Do error: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Errorf("CommonProvider->Updoad: io.ReadAll error: %v", err)
		return nil, err
	}

	logger.Debugf("CommonProvider->Updoad: body: %s", string(responseBody))
	logger.Debugf("CommonProvider->Updoad end")
	return responseBody, nil
}
