package dkvs

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/avast/retry-go"
	"github.com/gogo/protobuf/proto"
	badgerds "github.com/ipfs/go-ds-badger2"
	levelds "github.com/ipfs/go-ds-leveldb"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	kadpb "github.com/libp2p/go-libp2p-kad-dht/pb"
	"github.com/libp2p/go-libp2p/core/protocol"
	ldbopts "github.com/syndtr/goleveldb/leveldb/opt"
	tvCommon "github.com/tinyverse-web3/tvbase/common"
	tvConfig "github.com/tinyverse-web3/tvbase/common/config"
	"github.com/tinyverse-web3/tvbase/common/db"
	kaddht "github.com/tinyverse-web3/tvbase/dkvs/kaddht"
	pb "github.com/tinyverse-web3/tvbase/dkvs/pb"
)

type Dkvs struct {
	idht           *dht.IpfsDHT
	dkvsdb         db.Datastore // 对这个对象的操作要考虑加锁
	dhtDatastore   db.Datastore
	protoMessenger *kadpb.ProtocolMessenger
	baseService    tvCommon.TvBaseService
	baseServiceCfg *tvConfig.NodeConfig
}

func NewDkvs(tvbase tvCommon.TvBaseService) *Dkvs {
	rootPath := tvbase.GetConfig().RootPath
	dbPath := rootPath + string(filepath.Separator) + "unsynckv"
	dkvsdb, err := createBadgerDB(dbPath)
	if err != nil {
		Logger.Errorf("NewDkvs CreateDataStore: %v", err)
		return nil
	}
	idht := tvbase.GetDht()
	baseServiceCfg := tvbase.GetConfig()
	dhtDatastore := tvbase.GetDhtDatabase()
	pms, err := getProtocolMessenger(baseServiceCfg, idht)
	if err != nil {
		Logger.Errorf("NewDkvs getProtocolMessenger： %v", err)
		return nil
	}
	_dkvs := &Dkvs{
		idht:           idht,
		dkvsdb:         dkvsdb,
		dhtDatastore:   dhtDatastore,
		protoMessenger: pms,
		baseService:    tvbase,
		baseServiceCfg: baseServiceCfg,
	}

	// register a network event to handler unsynckey
	tvbase.RegistConnectedCallback(_dkvs.putAllUnsyncKeyToNetwork)
	return _dkvs
}

// sig 由发起者对key+val+pubkey+ttl的签名 (调用GetSignData)
func (d *Dkvs) Put(key string, val []byte, pubkey []byte, issuetime uint64, ttl uint64, sig []byte) error {
	if !isValidKey(key) {
		err := errors.New("invalid key")
		Logger.Error(err)
		return err
	}

	valueType := 0
	if IsGunName(key) {
		// 如果是/gun/name这样的格式，就检查内容，需要有GUN证书，并且保证证书不被随意覆盖(更长的就不检查了，只检查name是否有权限)
		valueType = 1
	}

	dr, err := CreateRecordWithType(val, pubkey, issuetime, ttl, sig, uint32(valueType))
	if err != nil {
		return err
	}

	return d.putRecord(key, dr)
}

// value, pubkey, issuetime, ttl, signature
func (d *Dkvs) Get(key string) ([]byte, []byte, uint64, uint64, []byte, error) {
	if !isValidKey(key) {
		err := errors.New("invalid key")
		Logger.Error(err)
		return nil, nil, 0, 0, nil, err
	}

	record, err := d.dhtGetRecord(RecordKey(key))
	if err != nil {
		return nil, nil, 0, 0, nil, err
	}

	return record.Value, record.PubKey, record.Validity - record.Ttl, record.Ttl, record.Signature, err
}

func (d *Dkvs) GetRecord(key string) (*pb.DkvsRecord, error) {
	if !isValidKey(key) {
		err := errors.New("invalid key")
		Logger.Error(err)
		return nil, err
	}

	return d.dhtGetRecord(RecordKey(key))
}

func (d *Dkvs) FastGetRecord(key string) (*pb.DkvsRecord, error) {
	if !isValidKey(key) {
		err := errors.New("invalid key")
		Logger.Error(err)
		return nil, err
	}
	// 需要实现一个快速查找，方便检查时调用，加快速度
	return d.dhtGetRecord(RecordKey(key))
}

func (d *Dkvs) Has(key string) bool {
	if !isValidKey(key) {
		err := errors.New("invalid key")
		Logger.Error(err)
		return false
	}
	_, err := d.idht.GetValue(context.Background(), RecordKey(key))
	return err == nil
}

func GetTtlFromDuration(t time.Duration) uint64 {
	return uint64(t.Milliseconds())
}

func GetDefaultTtl() uint64 {
	return uint64(DefaultDKVSRecordEOL.Milliseconds())
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

// sig1 由发起者对key+pubkey1+pubkey2的签名 (调用GetRecordSignData)
// record2 由发起者生成，并且由接受者签名的record记录，校验后可以直接调用dhtPut
func (d *Dkvs) TransferKey(key string, value1, pubkey1 []byte, sig1 []byte,
	value2 []byte, pubkey2 []byte, issuetime uint64, ttl uint64, sig2 []byte) error {
	if !isValidKey(key) {
		err := errors.New("invalid key")
		Logger.Error(err)
		return err
	}

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
	err = ValidateValue(key, value1, pubkey1, issuetime, ttl, sig1, int(pb.DkvsRecord_GUN_SIGNATURE))
	if err != nil {
		Logger.Error("ValidateValue ", err)
		return err
	}

	err = ValidateValue(key, value2, pubkey2, issuetime, ttl, sig2, _ValueType_Transfer)
	if err != nil {
		Logger.Error("ValidateValue ", err)
		return err
	}

	err = ValidateValue(key, value2, pubkey2, issuetime, ttl, sig2, int(pb.DkvsRecord_GUN_SIGNATURE))
	if err != nil {
		Logger.Error("ValidateValue ", err)
		return err
	}

	if !VerifyTransferCert(key, value1, pubkey2) {
		err = errors.New("VerifyTransferCert failed")
		Logger.Error(err)
		return err
	}

	recordKey := RecordKey(key)
	// 先放证书
	tmpRec2, err := CreateRecordWithType(value1, pubkey1, issuetime, ttl, sig1, uint32(pb.DkvsRecord_GUN_SIGNATURE))
	if err != nil {
		return err
	}

	tmpRecBuf2, err := tmpRec2.Marshal()
	if err != nil {
		return err
	}

	err = d.dhtPut(recordKey, tmpRecBuf2) // 必须同步到网络上
	if err != nil {
		Logger.Error(err)
		return err
	}
	// 转移
	tmpRec3, err := CreateRecordWithType(value2, pubkey2, issuetime, ttl, sig2, _ValueType_Transfer)
	if err != nil {
		return err
	}

	tmpRecBuf3, err := tmpRec3.Marshal()
	if err != nil {
		return err
	}
	err = d.dhtPut(recordKey, tmpRecBuf3) // 必须同步到网络上
	if err != nil {
		Logger.Error(err)
		return err
	}

	// 重新设置正确的标志位
	tmpRec4, err := CreateRecordWithType(value2, pubkey2, issuetime, ttl, sig2, uint32(pb.DkvsRecord_GUN_SIGNATURE))
	if err != nil {
		return err
	}

	tmpRecBuf4, err := tmpRec4.Marshal()
	if err != nil {
		return err
	}
	err = d.dhtPut(recordKey, tmpRecBuf4) // 必须同步到网络上
	if err != nil {
		Logger.Error(err)
		return err
	}

	return nil
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

func getProtocolMessenger(baseServiceCfg *tvConfig.NodeConfig, dht *dht.IpfsDHT) (*kadpb.ProtocolMessenger, error) {
	v1proto := baseServiceCfg.DHT.ProtocolPrefix + baseServiceCfg.DHT.ProtocolID
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

func isValidKey(key string) bool {
	if tl := len(key); tl > MaxDKVSRecordKeyLength || tl == 0 {
		Logger.Error("invalid length")
		return false
	}

	if key[0] != '/' {
		Logger.Error("invalid key")
		return false
	}

	return true
}

// key的格式：只关注前两个
// /namespace/name 或者 /namespace/name/xxx
func (d *Dkvs) checkKeyValidity(key string, pubkey []byte) bool {

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

func (d *Dkvs) dhtGetRecord(key string) (*pb.DkvsRecord, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var val []byte
	val, err := d.dhtGetRecordFromNet(ctx, key)
	if err != nil {
		Logger.Errorf("dhtGetRecord--> key does not exist on the network {key: %s, err: %v}", key, err)
		return nil, err
	}
	e := new(pb.DkvsRecord)
	if err := proto.Unmarshal(val, e); err != nil {
		Logger.Errorf("dhtGetRecord--> found an invalid dkvs entry: v%", err)
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

	d.baseService.GetAvailableServicePeerList(key) //增加Get的稳定性

	err := retry.Do(
		func() error {
			var err error
			val, err = d.idht.GetValue(ctx, key)
			return err
		},
		retryStrategy...,
	)
	if err != nil {
		Logger.Debugf("dhtGetRecordFromNet---> {key: %s, err: %v}", key, err)
		return nil, err
	}
	return val, nil
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
