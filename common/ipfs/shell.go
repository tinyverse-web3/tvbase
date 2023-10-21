package ipfs

import (
	"context"
	"io"

	shell "github.com/ipfs/go-ipfs-api"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
)

type IpfsShellProxy struct {
	sh  *shell.Shell
	url string
}

var ipfsShell *IpfsShellProxy

func CreateIpfsShellProxy(url string) (*IpfsShellProxy, error) {
	ret := &IpfsShellProxy{}
	err := ret.Init(url)
	if err != nil {
		return nil, err
	}
	ipfsShell = ret
	return ret, nil
}

func GetIpfsShellProxy() *IpfsShellProxy {
	return ipfsShell
}

func (s *IpfsShellProxy) Init(url string) error {
	s.url = url
	maddr, err := ma.NewMultiaddr(url)
	if err != nil {
		return err
	}
	_, _, err = manet.DialArgs(maddr)
	if err != nil {
		return err
	}
	s.sh = shell.NewShell(url)
	_, _, err = s.sh.Version()
	if err != nil {
		return err
	}
	return nil
}

func (s *IpfsShellProxy) Add(r io.Reader) (cid string, err error) {
	return s.sh.Add(r, shell.Pin(true), shell.CidVersion(1))
}

func (s *IpfsShellProxy) BlockGet(cid string) (block []byte, err error) {
	return s.sh.BlockGet(cid)
}

func (s *IpfsShellProxy) BlockPut(block []byte, format, mhtype string, mhlen int) (cid string, err error) {
	return s.sh.BlockPut(block, format, mhtype, mhlen)
}

func (s *IpfsShellProxy) BlockStat(cid string) (key string, size int, err error) {
	return s.sh.BlockStat(cid)
}

func (s *IpfsShellProxy) BlockPutVo(block []byte) (cid string, err error) {
	return s.BlockPut(block, "v0", "sha2-256", -1)
}

func (s *IpfsShellProxy) ObjectGet(cid string) (*shell.IpfsObject, error) {
	return s.sh.ObjectGet(cid)
}

func (s *IpfsShellProxy) ObjectPut(obj *shell.IpfsObject) (string, error) {
	return s.sh.ObjectPut(obj)
}

func (s *IpfsShellProxy) ObjectStat(cid string) (*shell.ObjectStats, error) {
	return s.sh.ObjectStat(cid)
}

func (s *IpfsShellProxy) IsPin(cid string) (isPin bool) {
	pinList, err := s.sh.Pins()
	if err != nil {
		return false
	}
	info := pinList[cid]
	return info.Type != ""
}

func (s *IpfsShellProxy) RecursivePin(cid string, ctx context.Context) error {
	return s.sh.Request("pin/add", cid).
		Option("recursive", true).
		Exec(ctx, nil)
}

func (s *IpfsShellProxy) DirectPin(cid string, ctx context.Context) error {
	return s.sh.Request("pin/add", cid).
		Option("direct", true).
		Exec(ctx, nil)
}

func (s *IpfsShellProxy) IndirectPin(cid string, ctx context.Context) error {
	return s.sh.Request("pin/add", cid).
		Option("indirect", true).
		Exec(ctx, nil)
}

func (s *IpfsShellProxy) Unpin(cid string) error {
	return s.sh.Unpin(cid)
}

func (s *IpfsShellProxy) Cat(cid string) (io.ReadCloser, error) {
	return s.sh.Cat(cid)
}

func (s *IpfsShellProxy) FindProviders(cid string, maxProviders int) (providerList []string, err error) {
	info := struct {
		ID        string
		Extra     string
		Type      int
		Responses []struct {
			Addrs []string
			ID    string
		}
	}{}

	err = s.sh.Request("routing/findprovs", cid).Option("num-providers", maxProviders).Exec(context.Background(), &info)
	if err != nil {
		return nil, err
	}
	for _, addr := range info.Responses {
		providerList = append(providerList, addr.ID)
	}

	return providerList, nil
}
