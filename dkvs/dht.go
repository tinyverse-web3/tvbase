package dkvs

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	// "github.com/avast/retry-go"
	"github.com/gogo/protobuf/proto"
	u "github.com/ipfs/boxo/util"
	"github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	record "github.com/libp2p/go-libp2p-record"
	recpb "github.com/libp2p/go-libp2p-record/pb"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/multiformats/go-base32"
	"github.com/tinyverse-web3/tvbase/dkvs/kaddht"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var (
	lock sync.Mutex
)

const (
	kad1 protocol.ID = "/kad/1.0.0"
)

func (d *Dkvs) asyncPut(key string, value []byte) (err error) {
	ctx1, cancel := context.WithCancel(context.Background())
	defer cancel()
	//先put到本地
	err = d.putKeyToLocalNode(ctx1, key, value)
	if err != nil {
		return err
	}
	//put本地成功后，然后再异步put到网络上去
	go func() {
		//执行异步操作
		ctx2, cancel := context.WithCancel(context.Background())
		defer cancel()
		rec, err := d.getLocal(ctx2, key)
		if err != nil {
			Logger.Errorf("asyncPut--->Failed to read key from local node {key: %s} err: %s", key, err.Error())
			return
		}
		if rec == nil {
			Logger.Errorf("asyncPut--->Failed to read key from local node {key: %s}", key)
			return
		}
		err = d.putKeyToNetNode(ctx2, key, rec)
		if err != nil && err.Error() == ErrLookupFailure.Error() {
			d.saveUnsyncedKey(ctx2, key) //Save the key to the local unsynchronized database
			Logger.Errorf("asyncPut--->put key to network failed, currently only put to the local node: {key: %s} at local", key)
		} else {
			Logger.Infof("asyncPut success! --->put key to network success, currently only put to the local node: {key: %} at local", key)
		}

		// TODO 下面代码待验证，检查key是否存在其他节点上
		// ctx2, cancel2 := context.WithCancel(context.Background())
		// defer cancel2()
		// var providers []peer.AddrInfo
		// providers, err = d.idht.ProviderStore().GetProviders(ctx2, []byte(key))
		// if err != nil {
		// 	Logger.Warn("key: %s is not saved to any other node", key)
		// }
		// Logger.Info("key in perr: ", providers)
		// providers := d.findPeersByKey(ctx2, key)
		// Logger.Infof("{key: %s} in perr: %v", key, providers)
	}()
	return nil
}

func (d *Dkvs) putKeyToLocalNode(ctx context.Context, key string, value []byte, opts ...routing.Option) (err error) {
	dht := d.idht
	ctx, span := d.startSpan(ctx, "IpfsDHT.PutValue", trace.WithAttributes(attribute.String("Key", key)))
	defer span.End()

	Logger.Debugw("putting value", "key", kaddht.LoggableRecordKeyString(key))
	Logger.Debug("putting value    dskey: ", d.mkDsKey(key))

	// don't even allow local users to put bad values.
	if err := dht.Validator.Validate(key, value); err != nil {
		return err
	}

	old, err := d.getLocal(ctx, key)
	if err != nil {
		// Means something is wrong with the datastore.
		return err
	}

	// Check if we have an old value that's not the same as the new one.
	if old != nil && !bytes.Equal(old.GetValue(), value) {
		// Check to see if the new one is better.
		i, err := dht.Validator.Select(key, [][]byte{value, old.GetValue()})
		if err != nil {
			return err
		}
		if i != 0 {
			return fmt.Errorf("can't replace a newer value with an older value")
		}
	}

	rec := record.MakePutRecord(key, value)
	rec.TimeReceived = u.FormatRFC3339(time.Now())
	err = d.putLocal(ctx, key, rec)
	if err != nil {
		return err
	}
	return nil
}

func (d *Dkvs) getLocal(ctx context.Context, key string) (*recpb.Record, error) {
	Logger.Debugw("finding value in datastore", "key", kaddht.LoggableRecordKeyString(key))

	rec, err := d.getRecordFromDatastore(ctx, key)
	if err != nil {
		Logger.Warnw("get local failed", "key", kaddht.LoggableRecordKeyString(key), "error", err)
		return nil, err
	}

	// Double check the key. Can't hurt.
	if rec != nil && string(rec.GetKey()) != key {
		Logger.Errorw("BUG: found a DHT record that didn't match it's key", "expected", kaddht.LoggableRecordKeyString(key), "got", rec.GetKey())
		return nil, nil

	}
	return rec, nil
}

func (d *Dkvs) getRecordFromDatastore(ctx context.Context, key string) (*recpb.Record, error) {
	dskey := d.mkDsKey(key)
	dht := d.idht
	buf, err := d.dhtDatastore.Get(ctx, dskey)
	if err == ds.ErrNotFound {
		Logger.Debug("finding value in datastore key: %s, error: ", dskey, "error: %s", err)
		return nil, nil
	}
	if err != nil {
		Logger.Errorw("error retrieving record from datastore", "key", dskey, "error", err)
		return nil, err
	}
	rec := new(recpb.Record)
	err = proto.Unmarshal(buf, rec)
	if err != nil {
		// Bad data in datastore, log it but don't return an error, we'll just overwrite it
		Logger.Errorw("failed to unmarshal record from datastore", "key", dskey, "error", err)
		return nil, nil
	}

	err = dht.Validator.Validate(string(rec.GetKey()), rec.GetValue())
	if err != nil {
		// Invalid record in datastore, probably expired but don't return an error,
		// we'll just overwrite it
		Logger.Debugw("local record verify failed", "key", rec.GetKey(), "error", err)
		return nil, nil
	}

	return rec, nil
}

// putLocal stores the key value pair in the datastore
func (d *Dkvs) putLocal(ctx context.Context, key string, rec *recpb.Record) error {
	data, err := proto.Marshal(rec)
	if err != nil {
		Logger.Warnw("failed to put marshal record for local put", "error", err, "key", kaddht.LoggableRecordKeyString(key))
		return err
	}

	return d.dhtDatastore.Put(ctx, d.mkDsKey(key), data)
}

func (d *Dkvs) startSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return otel.Tracer("go-libp2p-kad-dht").Start(ctx, fmt.Sprintf("KademliaDHT.%s", name), opts...)
}

func (d *Dkvs) putAllUnsyncKeyToNetwork(peerID peer.ID) error {
	lock.Lock() //加锁防止多线程同时操作putAllKeyToPeers
	defer lock.Unlock()
	log.Printf("peerID: %s", peerID.Pretty())
	d.putAllKeysToPeers() //将db中未同步的key再一次put到基他节点
	return nil
}

func (d *Dkvs) putKeyToNetNode(ctx context.Context, key string, rec *recpb.Record) error {
	peers, err := d.baseService.GetAvailableServicePeerList(key)
	// peers, err := d.idht.GetClosestPeers(ctx, key)
	// peers, err := d.getConnectedPeers(ctx, key)
	if err != nil {
		return err
	}
	if peers == nil {
		return fmt.Errorf("putKeyToNetNode--> not find any connected node")
	}
	wg := sync.WaitGroup{}
	for _, p := range peers {
		wg.Add(1)
		go func(p peer.ID) {
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			defer wg.Done()
			routing.PublishQueryEvent(ctx, &routing.QueryEvent{
				Type: routing.Value,
				ID:   p,
			})

			err := d.protoMessenger.PutValue(ctx, p, rec)
			if err != nil {
				Logger.Debugf("putKeyToNetNode--> failed putting value to peer: %s", err)
			}
		}(p)
	}
	wg.Wait()
	return nil
}

// 传入业务的key转换成数据库的key格式
func (d *Dkvs) mkDsKey(s string) ds.Key {
	return ds.NewKey(base32.RawStdEncoding.EncodeToString([]byte(s)))
}

// 传入数据库的key的字符串格式（dsk字符串格式）可以decode到原来的业务的key
func (d *Dkvs) mkDsKeyDcode(s string) ([]byte, error) {
	return base32.RawStdEncoding.DecodeString(s)
}

func (d *Dkvs) getUnsyncedKey(cxt context.Context, key string) ([]byte, error) {
	value, err := d.ldb.Get(cxt, mkDsKey(key))
	if err != nil {
		Logger.Error("LDB Get ", err)
		return nil, err
	}

	return value, nil
}

func (d *Dkvs) saveUnsyncedKey(cxt context.Context, key string) error {
	err := d.ldb.Put(cxt, d.mkDsKey(key), []byte(key))
	if err != nil {
		Logger.Error("Unsync LDB Put ", err)
	}
	d.printUnsyncedDb() //for debug
	return err
}

func (d *Dkvs) deleteUnsyncedKey(cxt context.Context, key string) error {
	err := d.ldb.Delete(cxt, d.mkDsKey(key))
	if err != nil {
		Logger.Error("Unsync LDB Delete ", err)
	}

	return err
}

func (d *Dkvs) closeUnsyncedDB(cxt context.Context) error {
	err := d.ldb.Close()
	if err != nil {
		Logger.Error("LDB Close ", err)
	}

	return err
}

func (d *Dkvs) printUnsyncedDb() {
	ctx := context.Background()
	q := query.Query{}
	results, err := d.ldb.Query(ctx, q)
	if err != nil {
		Logger.Errorf("printUnsyncDb---> Error querying DataStore: %v", err)
	}
	defer results.Close()

	for result := range results.Next() {
		Logger.Debugf("printUnsyncDb {key: %s}", result.Value)
	}
}

func (d *Dkvs) putAllKeysToPeers() error {
	// Query the DataStore for all keys and values
	ctx := context.Background()
	q := query.Query{}
	results, err := d.ldb.Query(ctx, q)
	if err != nil {
		Logger.Error("putAllKeysToPeers---> Error querying DataStore: ", err)
		return err
	}
	defer results.Close()

	delMap := make(map[string]string)
	for result := range results.Next() {
		Logger.Debugln("Key: ", result.Key)
		rec, err := d.getLocal(ctx, string(result.Value))
		if err != nil || rec == nil {
			Logger.Warn("putAllKeysToPeers---> There is no such key in dht db {key: %s}", result.Value)
			d.deleteUnsyncedKey(ctx, string(result.Value))
			continue
		}
		err = d.putKeyToNetNode(ctx, string(result.Value), rec)
		if err != nil {
			Logger.Debugf("putAllKeysToPeers---> dhtPut %s return %s\n", result.Value, err.Error())
		}
		if err == nil || err.Error() == ErrBadRecord.Error() {
			delMap[result.Key] = string(result.Value)
		}
	}

	for _, v := range delMap {
		err := d.deleteUnsyncedKey(ctx, v)
		if err != nil {
			Logger.Errorf("putAllKeysToPeers---> DeleteUnsyncKey %v", err.Error())
		}
	}
	Logger.Debug("putAllKeysToPeers---> printUnsyncedDb")
	d.printUnsyncedDb()
	return nil
}

func (d *Dkvs) findPeersByKey(ctx context.Context, key string) []peer.AddrInfo {
	ctxT, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	mhv := u.Hash([]byte(key))
	keyCid := cid.NewCidV1(cid.Raw, mhv)
	if !keyCid.Defined() {
		Logger.Error("invalid {cid: %s} undefined", keyCid)
		return nil
	}

	var providers []peer.AddrInfo
	// wg := sync.WaitGroup{}
	// wg.Add(1)

	// provs := d.idht.FindProvidersAsync(ctxT, keyCid, 1)
	// select {
	// case p, ok := <-provs:
	// 	if !ok {
	// 		Logger.Debug("findPeersByKey---> Provider channel was closed...")
	// 		break
	// 	}
	// 	if p.ID == "" {
	// 		Logger.Debug("findPeersByKey---> Got back nil provider!")
	// 	}
	// 	if p.ID != d.idht.PeerID() {
	// 		Logger.Debugf("findPeersByKey--> ygot a provider, but not the right one. %s", p)
	// 	}
	// 	providers = append(providers, p)
	// case <-ctxT.Done():
	// 	Logger.Debug("Didnt get back providers")
	// }
	// wg.Wait()
	providers, err := d.idht.FindProviders(ctxT, keyCid)
	if err != nil || providers == nil {
		Logger.Warnf("findPeersByKey--> {key: %s} is not saved to any other node", key)
	} else {
		Logger.Infof("findPeersByKey--> {key: %s} in perr: %v", key, providers)
	}
	return providers
}

// func (d *Dkvs) getConnectedPeers(ctx context.Context, key string) ([]peer.ID, error) {
// 	var peers []peer.ID
// 	retryStrategy := []retry.Option{
// 		retry.Delay(500 * time.Millisecond), // delay 500 ms
// 		retry.Attempts(3),                   // max retry times 3
// 		retry.LastErrorOnly(true),           // Return last error only
// 		retry.RetryIf(func(err error) bool { // Retries based on error type
// 			switch err.Error() {
// 			case ErrLookupFailure.Error():
// 				return true
// 			default:
// 				return false
// 			}
// 		}),
// 	}
// 	err := retry.Do(
// 		func() error {
// 			var err error
// 			peers, err = d.idht.GetClosestPeers(ctx, key) // 自定义的函数
// 			if err != nil {
// 				d.baseService.ConnectBootstrapNode()
// 			}
// 			return err // 返回错误
// 		},
// 		retryStrategy...,
// 	)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return peers, nil
// }