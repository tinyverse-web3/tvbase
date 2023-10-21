package storageprovider

import (
	"encoding/json"
	"fmt"
	"time"
)

// https://nft.storage/api-docs/

var (
	NftPostURL      = "https://api.nft.storage/upload"
	NftCheckCidURL  = "https://api.nft.storage/check"
	NftDeleteCidURL = "https://api.nft.storage"
)

type Nft struct {
	CommonProvider
}

func NewNft(apikey string) *Nft {
	nft := &Nft{}
	//TODO If it is larger than 100M, you need to use CAR to split it and upload it.
	maxSize := 100 * 1024 * 1024
	// the same API key exceeds 30 request within 10 seconds, the rate limit will be triggered
	intervalTime := 10 * time.Second
	var radio int64 = 30
	authKey := "Authorization"
	authValue := "Bearer " + apikey
	nft.Init(apikey, intervalTime, radio, maxSize, authKey, authValue)
	return nft
}

func (p *Nft) Upload(cid string, timeout time.Duration) (isOk bool, resp map[string]interface{}, err error) {
	responseData, err := p.CommonProvider.Upload(cid, timeout, NftPostURL)
	if err != nil {
		return false, nil, err
	}

	var data interface{}
	err = json.Unmarshal(responseData, &data)
	if err != nil {
		return false, nil, err
	}

	resp, ok := data.(map[string]interface{})
	if !ok {
		return false, nil, fmt.Errorf("Nft->Upload: failure to convert json object")
	}

	isOk, ok = resp["ok"].(bool)
	if !ok {
		return false, nil, fmt.Errorf("Nft->Upload: failure to get ok object, error: %v", resp)
	}

	return isOk, resp, nil
}

func (p *Nft) CheckCid(cid string) (isOk bool, resp map[string]interface{}, err error) {
	responseData, err := p.CommonProvider.CheckCid(cid, NftCheckCidURL)
	if err != nil {
		return false, nil, err
	}

	var data interface{}
	err = json.Unmarshal(responseData, &data)
	if err != nil {
		return false, nil, err
	}

	resp, ok := data.(map[string]interface{})
	if !ok {
		return false, nil, fmt.Errorf("Nft->CheckCid: failure to convert json object")
	}

	isOk, ok = resp["ok"].(bool)
	if !ok {
		return false, nil, fmt.Errorf("Nft->CheckCid: failure to get ok object, error: %v", resp)
	}
	return isOk, resp, nil
}

func (p *Nft) DeleteCid(cid string, postUrl string) (isOk bool, resp map[string]interface{}, err error) {
	responseData, err := p.CommonProvider.DeleteCid(cid, NftDeleteCidURL)
	if err != nil {
		return false, nil, err
	}

	var data interface{}
	err = json.Unmarshal(responseData, &data)
	if err != nil {
		return false, nil, err
	}

	resp, ok := data.(map[string]interface{})
	if !ok {
		return false, nil, fmt.Errorf("Nft->DeleteCid: failure to convert json object")
	}

	isOk, ok = resp["ok"].(bool)
	if !ok {
		return false, nil, fmt.Errorf("Nft->DeleteCid: failure to get ok object, error: %v", resp)
	}
	return isOk, resp, nil
}
