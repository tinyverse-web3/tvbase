package storageprovider

import (
	"encoding/json"
	"fmt"
	"time"
)

var (
	Web3PostURL     = "https://api.web3.storage/upload"
	Web3CarPostURL  = "https://api.web3.storage/car"
	Web3CheckCidURL = "https://api.nft.storage/check"
)

// https://web3.storage/docs/reference/http-api/
type Web3 struct {
	CommonProvider
}

func NewWeb3(apikey string) *Web3 {
	web3 := &Web3{}

	//TODO If it is larger than 100M, you need to use CAR to split it and upload it.
	maxSize := 100 * 1024 * 1024
	// the same API key exceeds 30 request within 10 seconds, the rate limit will be triggered
	intervalTime := 10 * time.Second
	var radio int64 = 30
	authKey := "Authorization"
	authValue := "Bearer " + apikey
	web3.Init(apikey, intervalTime, radio, maxSize, authKey, authValue)
	return web3
}

func (p *Web3) Upload(
	cid string,
	timeout time.Duration,
) (isOk bool, resp map[string]interface{}, err error) {
	responseData, err := p.CommonProvider.Upload(cid, timeout, Web3PostURL)
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
		return false, nil, fmt.Errorf("Web3->ParseResponse: failure to convert json object")
	}

	cid, ok = resp["cid"].(string)
	if !ok {
		return false, resp, fmt.Errorf("Web3->ParseResponse: failure to get cid object, error: %v", resp)
	}
	isOk = cid != ""
	return isOk, resp, nil
}

func (p *Web3) CheckCid(
	cid string,
) (isOk bool, resp map[string]interface{}, err error) {
	responseData, err := p.CommonProvider.CheckCid(cid, Web3CheckCidURL)
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
		return false, nil, fmt.Errorf("Web3->ParseResponse: failure to convert json object")
	}

	isOk, ok = resp["ok"].(bool)
	if !ok {
		return false, nil, fmt.Errorf("Web3->ParseResponse: failure to get ok object, error: %v", resp)
	}
	return isOk, resp, nil
}

func (p *Web3) UploadCar(cid string, timeout time.Duration) ([]byte, error) {
	//TODO call Web3CarPostURL
	return nil, nil
}
