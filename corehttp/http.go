package corehttp

import (
	"strconv"

	process "github.com/jbenet/goprocess"
	"github.com/tinyverse-web3/tvbase/common/define"
)

func StartWebService(t define.TvBaseService) {

	proc := process.WithParent(process.Background())
	addr := "/ip4/0.0.0.0/tcp/" + strconv.Itoa(t.GetConfig().CoreHttp.ApiPort)
	var opts = []ServeOption{
		QueryAllKeyOption(),
		QueryKeyOption(),
		QueryProviders(),
		QueryAllConnectdPeers(),
		QuerySystemResouce(),
		GetDemoKey(),
		GetEncryptedDemoValue(),
		PubDemoKey(),
		GetDemoKeyValue(),
	}
	Process = proc
	proc.Go(func(p process.Process) {
		if err := ListenAndServe(t, addr, opts...); err != nil {
			return
		}
	})
}
