package test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	tvIpfs "github.com/tinyverse-web3/tvbase/common/ipfs"
)

func TestNftStorageUpload(t *testing.T) {

	cid := "bafkreiawq4i3dlkubc7bwml5cchhnme7f4zft2rtv4b4ptrxhul2ehzmne"
	content, _, err := tvIpfs.IpfsBlockGet(cid, context.Background())
	if err != nil {
		return
	}
	fileBuffer := bytes.NewBuffer(content)
	if err != nil {
		testLog.Debugf("TestNftStorageUpload: writer.Close error: %v", err)
		return
	}
	client := &http.Client{}
	req, err := http.NewRequest("POST", "https://api.nft.storage/upload", fileBuffer)
	if err != nil {
		testLog.Errorf("TestNftStorageUpload: http.NewRequest error: %v", err)
		return
	}

	apikey := "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJkaWQ6ZXRocjoweDgxOTgwNzg4Y2UxQjY3MDQyM2Y1NzAyMDQ2OWM0MzI3YzNBNzU5YzciLCJpc3MiOiJuZnQtc3RvcmFnZSIsImlhdCI6MTY5MDUyODU1MjcxMywibmFtZSI6InRlc3QxIn0.vslsn8tAWUtZ0BZjcxhyMrcuufwfZ7fTMpF_DrojF4c"
	req.Header.Set("Authorization", apikey)

	resp, err := client.Do(req)
	if err != nil {
		testLog.Errorf("TestNftStorageUpload: client.Do error: %v", err)
		return
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		testLog.Errorf("TestNftStorageUpload: ioutil.ReadAll error: %v", err)
		return
	}

	testLog.Debugf("TestNftStorageUpload:: body: %s", string(responseBody))
}

func TestWeb3StorageUpload(t *testing.T) {

	cid := "bafkreiawq4i3dlkubc7bwml5cchhnme7f4zft2rtv4b4ptrxhul2ehzmne"
	content, _, err := tvIpfs.IpfsBlockGet(cid, context.Background())
	if err != nil {
		return
	}
	fileBuffer := bytes.NewBuffer(content)
	client := &http.Client{}
	req, err := http.NewRequest("POST", "https://api.web3.storage/upload", fileBuffer)
	if err != nil {
		testLog.Errorf("TestNftStorageUpload: http.NewRequest error: %v", err)
		return
	}

	apikey := "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJkaWQ6ZXRocjoweDAyYzZEYkJBMTQyOTA1MzliZjgwNkEzRkNDRDgzMDFmNWNjNTQ2ZDIiLCJpc3MiOiJ3ZWIzLXN0b3JhZ2UiLCJpYXQiOjE2OTA2ODkxMjg3NDUsIm5hbWUiOiJ0ZXN0In0.nhArwLJYjFwTiW1-SSRPyrCCczyYQ4T2PAHcShFZXqg"
	req.Header.Set("Authorization", apikey)

	resp, err := client.Do(req)
	if err != nil {
		testLog.Errorf("TestNftStorageUpload: client.Do error: %v", err)
		return
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		testLog.Errorf("TestNftStorageUpload: ioutil.ReadAll error: %v", err)
		return
	}

	testLog.Debugf("TestNftStorageUpload:: body: %s", string(responseBody))
}
