package test

import (
	"bytes"
	"context"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"testing"

	ipfsLog "github.com/ipfs/go-log/v2"
	tvIpfs "github.com/tinyverse-web3/tvbase/common/ipfs"
)

func TestNftStorageUpload(t *testing.T) {
	ipfsLog.SetLogLevel("tvbase_test", "debug")
	cid := "bafkreiawq4i3dlkubc7bwml5cchhnme7f4zft2rtv4b4ptrxhul2ehzmne"
	content, _, err := tvIpfs.IpfsBlockGet(cid, context.Background())
	if err != nil {
		return
	}

	requestBody := &bytes.Buffer{}
	writer := multipart.NewWriter(requestBody)
	fileWriter, err := writer.CreateFormFile("file", cid)
	if err != nil {
		testLog.Debugf("PullCidClientProtocol->HandleResponse: CreateFormFile error: %v", err)
		return
	}

	fileBuffer := bytes.NewBuffer(content)
	_, err = fileWriter.Write(fileBuffer.Bytes())
	if err != nil {
		testLog.Debugf("PullCidClientProtocol->HandleResponse: fileWriter.Write error: %v", err)
		return
	}
	writer.WriteField("Content-Type", writer.FormDataContentType())
	err = writer.Close()
	if err != nil {
		testLog.Debugf("PullCidClientProtocol->HandleResponse: writer.Close error: %v", err)
		return
	}

	client := &http.Client{}
	req, err := http.NewRequest("POST", "https://api.nft.storage/upload", requestBody)
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

	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		testLog.Errorf("TestNftStorageUpload: ioutil.ReadAll error: %v", err)
		return
	}

	testLog.Debugf("TestNftStorageUpload:: body: %s", string(responseBody))
}
