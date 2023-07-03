package dkvs

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/avast/retry-go"
	"github.com/gogo/protobuf/proto"
	u "github.com/ipfs/boxo/util"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	badgerds "github.com/ipfs/go-ds-badger2"
	levelds "github.com/ipfs/go-ds-leveldb"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	kadpb "github.com/libp2p/go-libp2p-kad-dht/pb"
	record "github.com/libp2p/go-libp2p-record"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/core/routing"
	ldbopts "github.com/syndtr/goleveldb/leveldb/opt"
	tvCommon "github.com/tinyverse-web3/tvbase/common"
	kaddht "github.com/tinyverse-web3/tvbase/dkvs/kaddht"
	pb "github.com/tinyverse-web3/tvbase/dkvs/pb"
	db "github.com/tinyverse-web3/tvutil/db"
)

// var logger *log.ZapEventLogger = log.Logger(DKVS_NAMESPACE)

type Dkvs struct {
	idht           *dht.IpfsDHT
	ldb            db.Datastore // 对这个对象的操作要考虑加锁
	dhtDatastore   db.Datastore
	protoMessenger *kadpb.ProtocolMessenger
	baseService    tvCommon.TvBaseService
}

/*
create the DKVS instance object
调用方式：

	var mtvNode sharenode.NodeInstance = node //将node实例传递给这个接口 通过接口是为了解决循环引用的问题及共享功能的规范化
	kv := dkvs.NewDkvs("./", mtvNode) //.表示当前路径 mtvNode是Node实例接口

Parameters:

	rootPath：dkvs数据库的保存的根路径
	node: mtvnode的实例接口

Returns:

	Dkvs实例
*/

func init() {
	// log.SetAllLoggers(log.LogDebug) // 如果要查看其他模块的全部日志，使用此开关
	// InitAPP(LogDebug)
	// InitModule(DKVS_NAMESPACE, LogDebug)
}

func NewDkvs(rootPath string, node tvCommon.TvBaseService) *Dkvs {
	dbPath := rootPath + string(filepath.Separator) + "unsynckv"
	ldb, err := createBadgerDB(dbPath)
	if err != nil {
		Logger.Error("NewDkvs CreateDataStore" + err.Error())
		return nil
	}
	idht := node.GetDht()
	pms, err := getProtocolMessenger(node, idht)
	if err != nil {
		Logger.Error("NewDkvs getProtocolMessenger" + err.Error())
		return nil
	}
	_dkvs := &Dkvs{
		idht:           idht,
		ldb:            ldb,
		dhtDatastore:   node.GetDhtDatabase(),
		protoMessenger: pms,
		baseService:    node,
	}

	// register a network event to handler unsynckey
	node.RegistConnectedCallback(_dkvs.putAllUnsyncKeyToNetwork)
	// node.RegistNetReachabilityChanged(_dkvs.putAllUnsyncKeyToNetwork)
	return _dkvs
}

// sig 由发起者对key+val+pubkey+ttl的签名 (调用GetSignData)
func (d *Dkvs) Put(key string, val []byte, pubkey []byte, issuetime uint64, ttl uint64, sig []byte) error {

	valueType := 0
	if IsGunService(pubkey) || IsGunName(key) {
		valueType = 1
	}

	dr, err := CreateRecordWithType(val, pubkey, issuetime, ttl, sig, uint32(valueType))
	if err != nil {
		return err
	}

	return d.putRecord(key, dr)
}

// value, pubkey, ttl, signature
func (d *Dkvs) Get(key string) ([]byte, []byte, uint64, uint64, []byte, error) {
	record, err := d.dhtGetRecord(RecordKey(key))
	if err != nil {
		return nil, nil, 0, 0, nil, err
	}

	return record.Value, record.PubKey, record.Validity - record.Ttl, record.Ttl, record.Signature, err
}

func (d *Dkvs) GetRecord(key string) (*pb.DkvsRecord, error) {
	return d.dhtGetRecord(RecordKey(key))
}

func (d *Dkvs) FastGetRecord(key string) (*pb.DkvsRecord, error) {
	// 需要实现一个快速查找，方便检查时调用，加快速度
	return d.dhtGetRecord(RecordKey(key))
}

func (d *Dkvs) Has(key string) bool {
	_, err := d.idht.GetValue(context.Background(), RecordKey(key))
	return err == nil
}

func (d *Dkvs) Delete(key string) bool {
	// set ttl to now + 1
	// need owner sign the value

	return false
}

func GetTtlFromDuration(t time.Duration) uint64 {
	return uint64(t.Milliseconds())
}

func TimeNow() uint64 {
	return uint64(time.Now().UnixMilli())
}

// used to sign with private key
func GetRecordSignData(key string, val []byte, pubkey []byte, issuetime uint64, ttl uint64) []byte {
	sigData := make([]byte, len(key)+len(val)+len(pubkey)+16)
	i := copy(sigData, []byte(key))
	i += copy(sigData[i:], val)
	i += copy(sigData[i:], pubkey)

	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, issuetime)
	i += copy(sigData[i:], b)
	binary.LittleEndian.PutUint64(b, ttl)
	copy(sigData[i:], b)

	return sigData
}

func GetRecordSignData2(key string, record *pb.DkvsRecord) []byte {
	return GetRecordSignData(key, record.Value, record.PubKey, record.Validity-record.Ttl, record.Ttl)
}

// sig1 由发起者对key+pubkey1+pubkey2的签名 (调用GetSignData)
// record2 由发起者生成，并且由接受者签名的record记录，校验后可以直接调用dhtPut
func (d *Dkvs) TransferKey(key string, pubkey1 []byte, sig1 []byte,
	value2 []byte, pubkey2 []byte, issuetime uint64, ttl uint64, sig2 []byte) error {

	// 检查发起者对key的所有权
	oldRec, err := d.dhtGetRecord(RecordKey(key))
	if err != nil {
		Logger.Error("dhtGetRecord ", err)
		return ErrTranferFailed
	}

	if !bytes.Equal(pubkey1, oldRec.PubKey) {
		Logger.Error("Equal key")
		return ErrTranferFailed
	}

	// 检查接受者数据的有效性
	if ttl != oldRec.Ttl ||
		!bytes.Equal(oldRec.Value, value2) {
		Logger.Error("Equal ttl or value")
		return ErrTranferFailed
	}

	// 提前检查数据的有效性
	err = ValidateValue(key, pubkey2, pubkey1, issuetime, ttl, sig1, 0)
	if err != nil {
		Logger.Error("ValidateValue ", err)
		return err
	}

	err = ValidateValue(key, value2, pubkey2, issuetime, ttl, sig2, 1)
	if err != nil {
		Logger.Error("ValidateValue ", err)
		return err
	}

	recordKey := RecordKey(key)
	// 已经检查过有效性，直接调用dhtPut
	tmpRec2, err := CreateRecordWithType(value2, pubkey2, issuetime, ttl, sig2, 1)
	if err != nil {
		return err
	}

	tmpRecBuf2, err := tmpRec2.Marshal()
	if err != nil {
		return err
	}
	// err = d.dhtPut(recordKey, tmpRecBuf2)
	err = d.asyncPut(recordKey, tmpRecBuf2)
	if err != nil {
		Logger.Error(err)
	}

	return err
}

func (d *Dkvs) putRecord(key string, record *pb.DkvsRecord) error {
	if record == nil {
		return ErrInvalidRecord
	}

	if !d.checkKeyValidity(key, record.PubKey) {
		Logger.Error(ErrInvalidPath)
		return ErrInvalidPath
	}

	// 提前检查数据的有效性，降低一个无效record造成的影响
	err := ValidateValue(key, record.Value, record.PubKey, record.Validity-record.Ttl, record.Ttl, record.Signature, int(record.ValueType))
	if err != nil {
		Logger.Error(err)
		return err
	}

	drMarsh, err := record.Marshal()
	if err != nil {
		return err
	}

	recordKey := RecordKey(key)
	//return d.dhtPut(recordKey, drMarsh)
	return d.asyncPut(recordKey, drMarsh)
}

func createLevelDB(dbRootDir string) (*levelds.Datastore, error) {
	fullPath := dbRootDir
	if !filepath.IsAbs(fullPath) {
		rootPath, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		fullPath = filepath.Join(rootPath, fullPath)
	}
	return levelds.NewDatastore(fullPath, &levelds.Options{
		Compression: ldbopts.NoCompression,
	})
}

func createBadgerDB(dbRootDir string) (*badgerds.Datastore, error) {
	fullPath := dbRootDir
	if !filepath.IsAbs(fullPath) {
		rootPath, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		fullPath = filepath.Join(rootPath, fullPath)
	}
	err := os.MkdirAll(fullPath, 0755)
	if err != nil {
		return nil, err
	}
	defopts := badgerds.DefaultOptions
	defopts.SyncWrites = false
	defopts.Truncate = true
	return badgerds.NewDatastore(fullPath, &defopts)
}

func getProtocolMessenger(baseService tvCommon.TvBaseService, dht *dht.IpfsDHT) (*kadpb.ProtocolMessenger, error) {
	// v1proto := "/tvnode" + kad1
	v1proto := baseService.GetConfig().DHT.ProtocolPrefix + string(kad1)
	//v1proto := protocol.ID(node.GetDhtProtocolPrefix()) + kad1
	protocols := []protocol.ID{protocol.ID(v1proto)}
	msgSender := kaddht.NewMessageSenderImpl(dht.Host(), protocols)
	return kadpb.NewProtocolMessenger(msgSender)
}

// len(name) <= MinDKVSRecordKeyLength
func (d *Dkvs) hasPermission(name string, pubkey []byte) bool {
	record, err := d.FastGetRecord(GetGunKey(name))
	if err != nil || !bytes.Equal(pubkey, record.PubKey) {
		return false
	}
	return true
}

func (d *Dkvs) checkNameValidity(name string, pubkey []byte) bool {
	len1 := len(name)
	if len1 == 0 {
		return true
	}
	if len1 <= MinDKVSRecordKeyLength {
		// 检查是否有权限
		if !d.hasPermission(name, pubkey) {
			Logger.Error("has no permission")
			return false
		}
	} else {
		if strings.HasPrefix(name, "0x") { // 公钥大概是74bytes
			// 对于每一个用公钥作为key的put，都检查是否由公钥owner签名，除非是特定的公共服务
			if name != bytesToHexString(pubkey) {
				Logger.Error("not self")
				return false
			}
		}
	}

	return true
}

// key的格式：只关注前两个
// /namespace/name 或者 /namespace/name/xxx
func (d *Dkvs) checkKeyValidity(key string, pubkey []byte) bool {

	if tl := len(key); tl > MaxDKVSRecordKeyLength || tl == 0 {
		Logger.Error("invalid length")
		return false
	}

	if key[0] != '/' {
		Logger.Error("invalid key")
		return false
	}

	subkeys := strings.Split(key, "/")
	if len(subkeys) < 3 { // subkeys[0] = ""
		Logger.Error("invalid key")
		return false
	}

	ns := subkeys[1]
	name := subkeys[2]
	len1 := len(ns)
	len2 := len(name)

	if len1 > MinDKVSRecordKeyLength && len2 > MinDKVSRecordKeyLength {
		return true
	}

	if d.IsPublicService(ns, pubkey) {
		return true
	} else if d.hasPermission(ns, pubkey) {
		return true
	}

	if d.checkNameValidity(name, pubkey) {
		return true
	}

	return false
}

// 检查下该key是否仅在本地（一般情况下，连上网络就自动同步上网络）
func (d *Dkvs) isAtLocal(key string) bool {
	ctx := context.Background()
	recordKey := RecordKey(key)
	value1, err1 := d.getUnsyncedKey(ctx, recordKey)
	if err1 == nil {
		value2, err2 := d.idht.GetValue(ctx, key)
		if err2 == nil && bytes.Equal(value1, value2) {
			return true
		}
	}

	return false
}

func mkDsKey(s string) datastore.Key {
	//return datastore.NewKey(base32.RawStdEncoding.EncodeToString([]byte(s)))
	return datastore.NewKey(s)
}

func (d *Dkvs) dhtPut(key string, value []byte) error {
	//ctxT, cancel := context.WithTimeout(context.Background(), 30*time.Second) //timeout is 30 second
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// err := idht.PutValue(ctx, key, value)
	//定义一个重试策略
	retryStrategy := []retry.Option{
		retry.Delay(500 * time.Millisecond), // delay 500 ms
		retry.Attempts(3),                   // max retry times 3
		retry.LastErrorOnly(true),           // Return last error only
		retry.RetryIf(func(err error) bool { // Retries based on error type
			switch err.Error() {
			case ErrLookupFailure.Error(), ErrNotFound.Error():
				return true
			default:
				return false
			}
		}),
	}
	err := retry.Do(
		func() error {
			err := d.idht.PutValue(ctx, key, value) // 自定义的函数
			return err                              // 返回错误
		},
		retryStrategy...,
	)
	if err != nil {
		return err
	}
	return nil
}

func (d *Dkvs) dhtPutValueToPeers(ctx context.Context, key string, value []byte) (err error) {

	// don't even allow local users to put bad values.
	if err := d.idht.Validator.Validate(key, value); err != nil {
		Logger.Error("Validate ", err)
		return ErrBadRecord
	}

	rec := record.MakePutRecord(key, value)
	rec.TimeReceived = u.FormatRFC3339(time.Now())

	peers, err := d.idht.GetClosestPeers(ctx, key)
	if err != nil {
		Logger.Error("GetClosestPeers ", err)
		return err
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

			// 需要修改dht，以便调用这个api
			//err := d.idht.protoMessenger.PutValue(ctx, p, rec)

			if err != nil {
				Logger.Error("failed putting value to peer: ", err)
			}
		}(p)
	}
	wg.Wait()

	return nil
}

func (d *Dkvs) dhtGetRecord(key string) (*pb.DkvsRecord, error) {
	//先从本地Get如果没有就从网络中获取
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var val []byte
	rec, err := d.getLocal(ctx, key)
	if err == nil && rec != nil { //因为d.getLocal当没有记录时rec会返回空且err也会返回空，所以除了判断err是否为空还需要加上rec是否空的判断
		val = rec.GetValue()
	} else {
		Logger.Warn("dhtGetRecord--> the local node does not have this {key: %s}")
		val, err = d.dhtGetRecordFromNet(ctx, key)
	}
	if err != nil {
		Logger.Error("dhtGetRecord--> neither the local node nor the network has this {key: %s}", key)
		return nil, err
	}
	e := new(pb.DkvsRecord)
	if err := proto.Unmarshal(val, e); err != nil {
		Logger.Error("dhtGetRecord--> found an invalid dkvs entry:", err)
		return nil, err
	}
	return e, nil
}

func (d *Dkvs) dhtGetRecordFromNet(ctx context.Context, key string) ([]byte, error) {
	var val []byte
	//定义一个重试策略
	retryStrategy := []retry.Option{
		retry.Delay(500 * time.Millisecond), // delay 500 ms
		retry.Attempts(3),                   // max retry times 3
		retry.LastErrorOnly(true),           // Return last error only
		retry.RetryIf(func(err error) bool { // Retries based on error type
			switch err.Error() {
			case ErrNotFound.Error():
				return false
			default:
				return true
			}
		}),
	}
	err := retry.Do(
		func() error {
			var err error
			val, err = d.idht.GetValue(ctx, key)
			return err
		},
		retryStrategy...,
	)
	if err != nil {
		return nil, err
	}
	return val, nil
}

func encodeLdbValue(redunt uint32) []byte {
	value := make([]byte, 4)
	binary.LittleEndian.PutUint32(value, redunt)
	return value
}

func decodeLdbValue(value []byte) uint32 {
	if len(value) < 4 {
		return 0
	}

	buf := make([]byte, 4)
	copy(buf, value[0:4])
	return binary.LittleEndian.Uint32(buf)
}

func (d *Dkvs) putAllKeyToPeers() error {
	// Query the DataStore for all keys and values
	ctx := context.Background()
	q := query.Query{}
	results, err := d.ldb.Query(ctx, q)
	if err != nil {
		Logger.Error("Error querying DataStore: ", err)
		return err
	}
	defer results.Close()

	delMap := make(map[string]bool)
	for result := range results.Next() {
		Logger.Debugln("Key: ", result.Key)
		Logger.Debugln("Value: ", result.Value)
		//err := d.dhtPutValueToPeers(ctx, result.Key, result.Value)
		err := d.dhtPut(result.Key, result.Value)
		Logger.Debugf("dhtPut %s return %s\n", result.Key, err)
		if err == nil || err.Error() == ErrBadRecord.Error() {
			delMap[result.Key] = true
		}
	}

	for k := range delMap {
		err := d.deleteUnsyncedKey(ctx, k)
		if err != nil {
			Logger.Error("DeleteUnsyncKey ", err)
		}
	}

	return nil
}

func (d *Dkvs) IsPublicService(sn string, pubkey []byte) bool {
	if len(sn) > MaxDKVPublicSNLength {
		return false
	}
	key, ok := dkvsServiceNameMap[sn]
	if ok {
		if key == bytesToHexString(pubkey) {
			return true
		} else {
			// 看看是否是派生出来的子公钥
			if d.IsChildPubkey(pubkey, hexStringToBytes(key)) {
				return true
			}
		}
	} else {
		if IsGunService(pubkey) {
			return true
		}

		// 看看是否是被授权的服务
		if d.IsApprovedService(sn) {
			return true
		}
	}

	return false
}

// 判断是否是子公钥，符合BIP32标准
func (d *Dkvs) IsChildPubkey(child []byte, parent []byte) bool {
	// TODO
	return false
}

func (d *Dkvs) IsApprovedService(sn string) bool {
	key := GetGunKey(sn)
	record, err := d.FastGetRecord(key)
	if err != nil {
		return false
	}

	rv := DecodeGunRecordValue(record.Value)
	if rv == nil {
		return false
	}

	cert := FindPublicServiceCertWithUserPubkey(rv.CertVect, record.PubKey)
	return cert != nil
}
