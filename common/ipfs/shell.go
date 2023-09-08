package ipfs

import (
	shell "github.com/ipfs/go-ipfs-api"
)

type IpfsShell struct {
	sh  *shell.Shell
	url string
}

var ipfsShell *IpfsShell

func CreateIpfsShell(url string) *IpfsShell {
	ret := &IpfsShell{}
	ret.Init(url)
	ipfsShell = ret
	return ret
}

func GetIpfsShell() *IpfsShell {
	return ipfsShell
}

func (s *IpfsShell) Init(url string) {
	s.url = url
	s.sh = shell.NewShell(url)
}

func (s *IpfsShell) BlockGet(cid string) (block []byte, err error) {
	return s.sh.BlockGet(cid)
}

func (s *IpfsShell) BlockPut(block []byte, format, mhtype string, mhlen int) (cid string, err error) {
	return s.sh.BlockPut(block, format, mhtype, mhlen)
}

func (s *IpfsShell) BlockStat(cid string) (key string, size int, err error) {
	return s.sh.BlockStat(cid)
}

func (s *IpfsShell) BlockPutVo(block []byte) (cid string, err error) {
	return s.BlockPut(block, "v0", "sha2-256", -1)
}

func (s *IpfsShell) ObjectGet(cid string) (*shell.IpfsObject, error) {
	return s.sh.ObjectGet(cid)
}

func (s *IpfsShell) ObjectPut(obj *shell.IpfsObject) (string, error) {
	return s.sh.ObjectPut(obj)
}

func (s *IpfsShell) ObjectStat(cid string) (*shell.ObjectStats, error) {
	return s.sh.ObjectStat(cid)
}

func (s *IpfsShell) IsPin(cid string) (isPin bool) {
	pinList, err := s.sh.Pins()
	if err != nil {
		return false
	}
	info := pinList[cid]
	return info.Type != ""
}

func (s *IpfsShell) Pin(cid string) error {
	return s.sh.Pin(cid)
}

func (s *IpfsShell) Unpin(cid string) error {
	return s.sh.Unpin(cid)
}
