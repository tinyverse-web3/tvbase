package corehttp

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/gogo/protobuf/proto"
	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	recpb "github.com/libp2p/go-libp2p-record/pb"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
	tvCommon "github.com/tinyverse-web3/tvbase/common"
	"github.com/tinyverse-web3/tvbase/dkvs"
	dkvs_pb "github.com/tinyverse-web3/tvbase/dkvs/pb"
)

type DkvsKV struct {
	Key      string `json:"key"`
	PutTime  string `json:"put_time"`
	Validity string `json:"validity"`
	PubKey   string `json:"pub_key"`
	Value    string `json:"value"`
}

type Node struct {
	NodeId   string `json:"node_id"`
	Addrs    string `json:"addrs"`
	PublicIp string `json:"public_ip"`
}

type SysRes struct {
	CpuInfo CpuInfo `json:"cpu_info"`
	Mem     MemInfo `json:"mem"`
	SysDisk SysDisk `json:"sys_disk"`
	AppDir  AppDir  `json:"app_dir"`
}

type AppDir struct {
	Path        string `json:"path"`
	FileCount   string `json:"file_count"`
	DirSize     string `json:"dir_size"`
	UsedPercent string `json:"used_percent"`
}

type SysDisk struct {
	Fstype      string `json:"fstype"`
	Total       string `json:"total"`
	Free        string `json:"free"`
	Used        string `json:"used"`
	UsedPercent string `json:"used_percent"`
}

type MemInfo struct {
	MemUsage    string `json:"memory_usage"`
	MemPercent  string `json:"memory_percent"`
	SysTotalMem string `json:"memory_total"`
}

type KvInfo struct {
	LocalKV *DkvsKV `json:"local_kv"`
	NetKV   *DkvsKV `json:"net_kv"`
}

type CpuInfo struct {
	CpuPercent string         `json:"cpu_percent"`
	CpuDetails []cpu.InfoStat `json:"cpu_details"`
}

func QueryAllKeyOption() ServeOption {
	return func(t tvCommon.TvBaseService, _ net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {
		db := t.GetDhtDatabase()
		ctx := context.Background()
		mux.HandleFunc("/tvbase/queryAllKeys", func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if r := recover(); r != nil {
					Logger.Errorf("QueryAllKeyOption-mux.HandleFunc: recovered from: %v", r)
				}
			}()
			q := query.Query{}
			results, err := db.Query(ctx, q)
			if err != nil {
				Logger.Errorf("queryAllKeys---> failed to querying dht datastore: %v", err)
				handleError(w, "failed to querying dht datastore", err, 400)
				return
			}
			defer results.Close()

			var keyList []DkvsKV
			for result := range results.Next() {
				keyObj := ds.NewKey(result.Key)
				key, err := dsKeyDcode(keyObj.List()[0])
				if err != nil {
					Logger.Debugf("queryAllKeys---> dsKeyDcode(keyObj.List()[0] failed: %v", err)
					continue
				}
				lbp2pRec := new(recpb.Record)
				err = proto.Unmarshal(result.Value, lbp2pRec)
				if err != nil {
					Logger.Debugf("queryAllKeys---> proto.Unmarshal(lbp2pRec.Value, lbp2pRec) failed: %v", err)
					continue
				}
				dkvsRec := new(dkvs_pb.DkvsRecord)
				if err := proto.Unmarshal(lbp2pRec.Value, dkvsRec); err != nil {
					Logger.Debugf("queryAllKeys---> proto.Unmarshal(dkvsRec.Value, dkvsRec) failed: %v", err)
					continue
				}
				var kv DkvsKV
				kv.Key = string(dkvs.RemovePrefix(string(key)))
				kv.Value = string(dkvsRec.Value)
				kv.PutTime = formatUnixTime(dkvsRec.Seq)
				kv.Validity = formatUnixTime(dkvsRec.Validity)
				kv.PubKey = hex.EncodeToString(dkvsRec.PubKey)
				keyList = append(keyList, kv)
			}
			jsonData, err := json.Marshal(keyList)
			if err != nil {
				Logger.Errorf("queryAllKeys---> json.Marshal(keyList) failed: %v", err)
				handleError(w, "failed to marshal keylist", err, 400)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, err = w.Write(jsonData)
			if err != nil {
				Logger.Errorf("queryAllKeys---> failed to write json data to w http.ResponseWriter: %v", err)
				handleError(w, "failed to write json data to w http.ResponseWriter", err, 400)
				return
			}
		})
		return mux, nil
	}
}

func QueryKeyOption() ServeOption {
	return func(t tvCommon.TvBaseService, _ net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {
		dkvsService := t.GetDkvsService()
		db := t.GetDhtDatabase()
		ctx := context.Background()
		mux.HandleFunc("/tvbase/queryKey", func(w http.ResponseWriter, r *http.Request) {
			kvi := new(KvInfo)
			queryParams := r.URL.Query()
			queryKey := queryParams.Get("key")
			var localValue *DkvsKV
			var netValue *DkvsKV
			var keyList []DkvsKV
			if len(queryKey) == 0 {
				Logger.Errorf("queryKey---> key does not exist in the url parameter: %v", fmt.Errorf("error"))
				handleError(w, "key does not exist in the url parameter", fmt.Errorf("error"), 400)
				return
			}
			dkvsKey := dkvs.RecordKey(queryKey) //convert to dkvs key
			//From Local
			results, err := db.Get(ctx, mkDsKey(dkvsKey))
			if err != nil {
				Logger.Errorf("queryKey---> key does not exist in dht datastore: %v", err)
				//handleError(w, "key does not exist in dht datastore", err, 400)
				localValue = &DkvsKV{
					Value: fmt.Sprintf("key does not exist in dht datastore: %s", err.Error()),
				}
			} else {
				localValue, err = getKVDetailsFromLibp2pRec(queryKey, results)
				if err != nil {
					Logger.Errorf("queryKey---> failed to call getKVDetailsFromLibp2pRec(results): %s", err.Error())
					//handleError(w, "failed to call getValueFromRec(results)", err, 400)
					localValue = &DkvsKV{
						Value: fmt.Sprintf("failed to call getKVDetailsFromLibp2pRec(queryKey, results): %s", err.Error()),
					}
				}
			}
			localValue.Key = "local: " + queryKey
			kvi.LocalKV = localValue
			keyList = append(keyList, *localValue)

			netResults, err := dkvsService.GetRecord(queryKey)
			if err != nil {
				Logger.Errorf("queryKey---> failed to call  dkvsService.GetRecord(queryKey): %s", err.Error())
				//handleError(w, "failed to call  dkvsService.GetRecord(queryKey)", err, 400)
				netValue = &DkvsKV{
					Value: fmt.Sprintf("failed to call  dkvsService.GetRecord(queryKey): %s", err.Error()),
				}
			} else {
				netValue, err = getKVDetailsFromDkvsRec(queryKey, netResults)
				if err != nil {
					Logger.Errorf("queryKey---> failed to call getValueFromRec(netResults): %s", err.Error())
					//handleError(w, "failed to call getValueFromRec(netResults)", err, 400)
					netValue = &DkvsKV{
						Value: fmt.Sprintf("failed to call getKVDetailsFromDkvsRec(queryKey, netResults): %s", err.Error()),
					}
				}
			}
			netValue.Key = "network: " + queryKey
			kvi.NetKV = netValue
			keyList = append(keyList, *netValue)

			jsonData, err := json.Marshal(keyList)
			if err != nil {
				Logger.Errorf("queryKey---> json.Marshal(queryKey) failed:  %v", err)
				handleError(w, "failed to marshal queryKey", err, 400)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, err = w.Write(jsonData)
			if err != nil {
				Logger.Errorf("queryKey---> failed to write json data to w http.ResponseWriter:  %v", err)
				handleError(w, "failed to write json data to w http.ResponseWriter", err, 400)
				return
			}
		})
		return mux, nil
	}
}

func QueryProviders() ServeOption {
	return func(t tvCommon.TvBaseService, _ net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {
		ctx := context.Background()
		kvSevice := t.GetDkvsService()
		mux.HandleFunc("/tvbase/queryProviders", func(w http.ResponseWriter, r *http.Request) {
			queryParams := r.URL.Query()
			queryKey := queryParams.Get("key")
			if len(queryKey) == 0 {
				Logger.Errorf("queryProviders---> key does not exist in the url parameter:  %v", fmt.Errorf("error"))
				handleError(w, "key does not exist in the url parameter", fmt.Errorf("error"), 400)
				return
			}
			peers := kvSevice.FindPeersByKey(ctx, queryKey, 5*time.Second)
			var nodeList []Node
			for _, peer := range peers {
				var node Node
				node.NodeId = peer.ID.Pretty()
				node.Addrs = peer.String()
				nodeList = append(nodeList, node)
			}
			jsonData, err := json.Marshal(nodeList)
			if err != nil {
				Logger.Errorf("queryProviders---> json.Marshal(nodeList) failed:  %v", err)
				handleError(w, "failed to marshal nodeList", err, 400)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, err = w.Write(jsonData)
			if err != nil {
				Logger.Errorf("queryProviders---> failed to write json data to w http.ResponseWriter:  %v", err)
				handleError(w, "failed to write json data to w http.ResponseWriter", err, 400)
				return
			}
		})
		return mux, nil
	}
}

func QueryAllConnectdPeers() ServeOption {
	return func(t tvCommon.TvBaseService, _ net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {
		mux.HandleFunc("/tvbase/queryAllPeers", func(w http.ResponseWriter, r *http.Request) {
			host := t.GetHost()
			peerstore := host.Peerstore()
			//peers := peerstore.Peers()
			peers := host.Network().Peers()
			var nodeList []Node
			for _, peerId := range peers {
				var node Node
				addrInfo := peerstore.PeerInfo(peerId)
				isPrivateIP, publicIp := isPrivateNode(addrInfo)
				if isPrivateIP {
					continue
				}
				node.NodeId = peerId.Pretty()
				node.Addrs = addrInfo.String()
				node.PublicIp = publicIp
				nodeList = append(nodeList, node)
			}
			jsonData, err := json.Marshal(nodeList)
			if err != nil {
				Logger.Errorf("queryAllPeers---> json.Marshal(nodeList) failed:  %v", err)
				handleError(w, "failed to marshal nodeList", err, 400)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, err = w.Write(jsonData)
			if err != nil {
				Logger.Errorf("queryAllPeers---> failed to write json data to w http.ResponseWriter:  %v", err)
				handleError(w, "failed to write json data to w http.ResponseWriter", err, 400)
				return
			}
		})
		return mux, nil
	}
}

func QuerySystemResouce() ServeOption {
	return func(t tvCommon.TvBaseService, _ net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {
		mux.HandleFunc("/tvbase/querySysRes", func(w http.ResponseWriter, r *http.Request) {
			ctx := t.GetCtx()
			var sysRes = new(SysRes)
			p, err := process.NewProcess(int32(os.Getpid()))
			if err != nil {
				Logger.Errorf("Failed to create process: %v", err)
				return
			}

			//CPU
			var cpuInfo = new(CpuInfo)
			cpuPercent, err := p.CPUPercentWithContext(ctx)
			if err != nil {
				Logger.Errorf("Failed to get CPU percent: %v", err)
				return
			}
			cpuInfo.CpuPercent = humanize.FtoaWithDigits(cpuPercent, 2)
			cpuDetails, err := cpu.Info()
			if err != nil {
				Logger.Errorf("Failed to get CPU Details: %v", err)
				return
			}
			totalCores := 0
			for _, info := range cpuDetails {
				totalCores += int(info.Cores)
			}
			cpuDetails[0].Cores = int32(totalCores)
			cpuInfo.CpuDetails = cpuDetails
			sysRes.CpuInfo = *cpuInfo

			//Memory
			mi, err := p.MemoryInfoWithContext(ctx)
			if err != nil {
				Logger.Errorf("Failed to get memory info: %v", err)
				return
			}
			memPercent, err := p.MemoryPercentWithContext(ctx)
			if err != nil {
				Logger.Errorf("Failed to get CPU percent: %v", err)
				return
			}
			machineMemory, err := mem.VirtualMemoryWithContext(ctx)
			if err != nil {
				Logger.Errorf("Failed to get sys total mem: %v", err)
				return
			}
			totalMem := machineMemory.Total
			var memInfo = new(MemInfo)
			memInfo.MemUsage = humanize.Bytes(mi.RSS)
			memInfo.MemPercent = humanize.FtoaWithDigits(float64(memPercent), 2)
			memInfo.SysTotalMem = humanize.Bytes(totalMem)
			sysRes.Mem = *memInfo

			//Disk
			dataPath := getTvBaseDataDir()
			var sysDisk = new(SysDisk)
			sysDiskStat, err := disk.UsageWithContext(ctx, dataPath)
			if err != nil {
				Logger.Errorf("Failed to get disk usage info: %v", err)
				return
			}
			sysDisk.Fstype = sysDiskStat.Fstype
			sysDisk.Free = humanize.Bytes(sysDiskStat.Free)
			sysDisk.Total = humanize.Bytes(sysDiskStat.Total)
			sysDisk.Used = humanize.Bytes(sysDiskStat.Used)
			sysDisk.UsedPercent = humanize.FtoaWithDigits(float64(sysDiskStat.UsedPercent), 2)
			sysRes.SysDisk = *sysDisk

			//AppDir
			var appDir = new(AppDir)
			dirSize, fileCount := getDirSizeAndFileCount(dataPath)
			dirUsedPercent := float64(dirSize) / float64(sysDiskStat.Total) * 100
			appDir.Path = dataPath
			appDir.DirSize = humanize.Bytes(uint64(dirSize))
			appDir.FileCount = fmt.Sprint(fileCount)
			appDir.UsedPercent = fmt.Sprintf("%.3f", dirUsedPercent)
			sysRes.AppDir = *appDir

			jsonData, err := json.Marshal(sysRes)
			if err != nil {
				Logger.Errorf("querySysRes---> json.Marshal(sysRes) failed: %v", err)
				handleError(w, "failed to marshal sysRes", err, 400)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, err = w.Write(jsonData)
			if err != nil {
				Logger.Errorf("querySysRes---> failed to write json data to w http.ResponseWriter: %v", err)
				handleError(w, "failed to write json data to w http.ResponseWriter", err, 400)
				return
			}
		})
		return mux, nil
	}
}

func handleError(w http.ResponseWriter, msg string, err error, code int) {
	http.Error(w, fmt.Sprintf("%s: %s", msg, err), code)
}
