package storageprovider

type UploadManager struct {
	NftUploaderList  []*Nft
	Web3UploaderList []*Web3
}

func NewUploaderManager() *UploadManager {
	ret := &UploadManager{
		NftUploaderList:  make([]*Nft, 0),
		Web3UploaderList: make([]*Web3, 0),
	}
	return ret
}

func (p *UploadManager) CleanNftUploader() {
	p.NftUploaderList = make([]*Nft, 0)
}

func (p *UploadManager) AddNftUploader(apikey string) {
	p.NftUploaderList = append(p.NftUploaderList, NewNft(apikey))
}

func (p *UploadManager) CleanWeb3Uploader() {
	p.Web3UploaderList = make([]*Web3, 0)
}

func (p *UploadManager) AddWeb3Uploader(apikey string) {
	p.Web3UploaderList = append(p.Web3UploaderList, NewWeb3(apikey))
}
