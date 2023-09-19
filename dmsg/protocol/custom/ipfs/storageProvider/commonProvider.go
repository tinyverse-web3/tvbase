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
	uploadMaxSize   int
	authKey         string
	authValue       string
	ratelimitBucket *ratelimit.Bucket
}

func (p *CommonProvider) Init(
	apikey string,
	intervalTime time.Duration,
	ratio int64,
	uploadMaxSize int,
	authKey string,
	authValue string,
) {
	p.uploadMaxSize = uploadMaxSize
	p.authKey = authKey
	p.authValue = authValue
	p.ratelimitBucket = ratelimit.NewBucket(intervalTime, ratio)
}

func (p *CommonProvider) Upload(cid string, timeout time.Duration, postUrl string) ([]byte, error) {
	logger.Debugf("CommonProvider->Updoad begin: cid: %s", cid)

	p.ratelimitBucket.Wait(1)

	sh := tvbaseIpfs.GetIpfsShellProxy()
	reader, err := sh.Cat(cid)
	if err != nil {
		logger.Errorf("CommonProvider->Updoad: sh.Cat error: %v", err)
		return nil, err
	}
	defer reader.Close()

	req, err := http.NewRequest("POST", postUrl, reader)
	if err != nil {
		logger.Errorf("CommonProvider->Updoad: http.NewRequest error: %v", err)
		return nil, nil
	}

	req.Header.Set(p.authKey, p.authValue)

	_, size, err := sh.BlockStat(cid)
	if err != nil {
		logger.Errorf("CommonProvider->Updoad: sh.BlockStat error: %v", err)
		return nil, err
	}

	if size >= p.uploadMaxSize {
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

func (p *CommonProvider) CheckCid(cid string, postUrl string) ([]byte, error) {
	logger.Debugf("CommonProvider->CheckCid begin: cid: %s", cid)

	p.ratelimitBucket.Wait(1)

	req, err := http.NewRequest("GET", postUrl+"/"+cid, nil)
	if err != nil {
		logger.Errorf("CommonProvider->CheckCid: http.NewRequest error: %v", err)
		return nil, nil
	}

	req.Header.Set(p.authKey, p.authValue)

	client := &http.Client{
		Timeout: CommonTimeout,
	}
	resp, err := client.Do(req)
	if err != nil {
		logger.Errorf("CommonProvider->CheckCid: client.Do error: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Errorf("CommonProvider->CheckCid: io.ReadAll error: %v", err)
		return nil, err
	}

	logger.Debugf("CommonProvider->CheckCid: body: %s", string(responseBody))
	logger.Debugf("CommonProvider->CheckCid end")
	return responseBody, nil
}

func (p *CommonProvider) DeleteCid(cid string, postUrl string) ([]byte, error) {
	logger.Debugf("CommonProvider->DeleteCid begin: cid: %s", cid)

	p.ratelimitBucket.Wait(1)

	req, err := http.NewRequest("DELETE", postUrl+"/"+cid, nil)
	if err != nil {
		logger.Errorf("CommonProvider->DeleteCid: http.NewRequest error: %v", err)
		return nil, nil
	}

	req.Header.Set(p.authKey, p.authValue)

	client := &http.Client{
		Timeout: CommonTimeout,
	}
	resp, err := client.Do(req)
	if err != nil {
		logger.Errorf("CommonProvider->DeleteCid: client.Do error: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Errorf("CommonProvider->DeleteCid: io.ReadAll error: %v", err)
		return nil, err
	}

	logger.Debugf("CommonProvider->DeleteCid: body: %s", string(responseBody))
	logger.Debugf("CommonProvider->DeleteCid end")
	return responseBody, nil
}
