package dkvs

import (
	"bytes"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	pb "github.com/tinyverse-web3/tvbase/dkvs/pb"
)

// 证书结构：dkvs.pb.Cert
// 证书由发行者记录在用户公钥下面，一般格式（路径可以自定义）
// addr: /服务名称/用户公钥/cert
// value: CertsRecordValue

var DefaultCertTtl = uint64(time.Duration(time.Hour * 24 * 365 * 100).Milliseconds())
const CertTransferData = "transfer"

func VerifyCert(cert *pb.Cert) bool {

	if cert == nil {
		return false
	}

	data := GetCertSignData(cert)

	issuepk, err := crypto.UnmarshalPublicKey(cert.IssuerPubkey)
	if err != nil {
		return false
	}

	if ok, err := issuepk.Verify(data, cert.IssuerSign); err != nil || !ok {
		return false
	}

	// check validaty
	if cert.Ttl != 0 {
		return TimeNow() <= cert.IssueTime+cert.Ttl
	}

	return true
}

func GetCertSignData(cert *pb.Cert) []byte {
	toSign := *cert
	toSign.IssuerSign = nil

	b, err := toSign.Marshal()
	if err != nil {
		return nil
	}

	return b
}

func EncodeCert(cert *pb.Cert) []byte {
	if cert == nil {
		return nil
	}

	b, err := cert.Marshal()
	if err != nil {
		return nil
	}

	return b
}

func DecodeCert(value []byte) *pb.Cert {

	var cert pb.Cert
	err := cert.Unmarshal(value)
	if err != nil {
		return nil
	}

	return &cert
}

func SearchCertByPubkey(cv []*pb.Cert, pubkey []byte) *pb.Cert {
	for _, c := range cv {
		if bytes.Equal(pubkey, c.IssuerPubkey) {
			return c
		}
	}
	return nil
}

func SearchCertByName(cv []*pb.Cert, name string) *pb.Cert {
	for _, c := range cv {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func IsCertExisted(cv []*pb.Cert, cert *pb.Cert) bool {
	for _, c := range cv {
		if c.IssueTime == cert.IssueTime && c.Ttl == cert.Ttl && c.Version == cert.Version &&
			c.Name == cert.Name && c.Type == cert.Type &&
			bytes.Equal(c.UserPubkey, cert.UserPubkey) && bytes.Equal(c.Data, cert.Data) &&
			bytes.Equal(c.IssuerSign, cert.IssuerSign) && bytes.Equal(c.IssuerPubkey, cert.IssuerPubkey) {
			return true
		}
	}
	return false
}

func DecodeAndFindCertByPubkey(value []byte, pubkey []byte) *pb.Cert {
	rv := DecodeGunRecordValue(value)
	if rv == nil {
		return nil
	}

	return SearchCertByPubkey(rv.CertVect, pubkey)
}

// used to sign with private key
func GetGunSignData(name string, gunPubkey []byte, issueTime uint64, ttl uint64) []byte {

	cert := pb.Cert{
		Version:      1,
		Name:         KEY_NS_GUN,
		Type:         uint32(pb.CertType_Default),
		SubType: 	  0,
		UserPubkey:   nil,
		Data:         []byte(name),
		IssueTime:    issueTime,
		Ttl:          ttl,
		IssuerPubkey: gunPubkey,
		IssuerSign:   nil,
	}

	b, err := cert.Marshal()
	if err != nil {
		return nil
	}

	return b
}

// generate value for a GUN record
func EncodeGunValue(name string, issueTime uint64, ttl uint64, gunPubkey []byte, gunSign []byte, userData []byte) []byte {
	cert := pb.Cert{
		Version:      1,
		Name:         KEY_NS_GUN,
		Type:         uint32(pb.CertType_Default),
		SubType: 	  0,
		UserPubkey:   nil,
		Data:         []byte(name),
		IssueTime:    issueTime,
		Ttl:          ttl,
		IssuerPubkey: gunPubkey,
		IssuerSign:   gunSign,
	}

	rv := pb.CertsRecordValue{
		UserData: userData,			// 在转移时放一个证书
		CertVect: []*pb.Cert{&cert},
	}

	b, err := rv.Marshal()
	if err != nil {
		return nil
	}

	return b
}

// decode from value by a GUN record
func DecodeGunRecordValue(value []byte) *pb.CertsRecordValue {
	var rv pb.CertsRecordValue
	err := rv.Unmarshal(value)
	if err != nil {
		return nil
	}

	return &rv
}

func VerifyGunRecordValue(key string, value []byte, issuetime uint64, ttl uint64) bool {

	rv := DecodeGunRecordValue(value)
	if rv == nil {
		return false
	}

	cert := FindGunCert(rv.CertVect)
	if cert == nil {
		return false
	}

	return cert.Ttl == ttl && cert.IssueTime == issuetime && GetGunKey(string(cert.Data)) == key
}


func FindTransferCert(cv []*pb.Cert) *pb.Cert {
	cert := SearchCertByName(cv, CertTransferData)
	if IsTransferCert(cert) {
		return cert
	}
	return nil
}

func IsTransferCert(cert *pb.Cert) bool {
	if cert != nil && VerifyCert(cert) {
		return true
	}
	return false
}


func VerifyTransferRecordValue(key string, value []byte, pk []byte) bool {

	rv := DecodeCertsRecordValue(value)
	if rv == nil {
		return false
	}
	var cert *pb.Cert
	if rv.UserData != nil {
		// try to decode it
		cert = DecodeCert(rv.UserData)
		if !IsTransferCert(cert) {
			cert = nil
		}
	}

	if cert == nil {
		cert = FindTransferCert(rv.CertVect)
		if cert == nil {
			return false
		}
	}

	return bytes.Equal(cert.IssuerPubkey, pk) && string(cert.Data) == key
}


func VerifyTransferCert(key string, oldvalue []byte, newpk []byte) bool {
	// 
	oldcerts := DecodeCertsRecordValue(oldvalue)
	if oldcerts == nil {
		return false
	}
	if oldcerts.UserData == nil {
		return false
	}
	cert1 := DecodeCert(oldcerts.UserData)
	if cert1 == nil {
		return false
	}
	if !VerifyCert(cert1) {
		return false
	}
	if !bytes.Equal(cert1.UserPubkey, newpk) || cert1.Name != CertTransferData || string(cert1.Data) != key {
		return false
	}

	return true
}

// used to sign with private key
func IssueCert(name string, data []byte, issuePubkey []byte) *pb.Cert {

	cert := pb.Cert{
		Version:      1,
		Name:         name,
		Type:         uint32(pb.CertType_Default),
		SubType: 	  0,
		UserPubkey:   nil,
		Data:         data,
		IssueTime:    TimeNow(),
		Ttl:          DefaultCertTtl,
		IssuerPubkey: issuePubkey,
		IssuerSign:   nil,
	}

	return &cert
}


// used to sign with private key
func IssueTransferCert(key string, receiverpk, issuePubkey []byte) *pb.Cert {

	cert := pb.Cert{
		Version:      1,
		Name:         CertTransferData,
		Type:         uint32(pb.CertType_Default),
		SubType: 	  0,
		UserPubkey:   receiverpk,
		Data:         []byte(key),
		IssueTime:    TimeNow(),
		Ttl:          DefaultCertTtl,
		IssuerPubkey: issuePubkey,
		IssuerSign:   nil,
	}

	return &cert
}



// decode from value
func DecodeCertsRecordValue(value []byte) *pb.CertsRecordValue {
	var rv pb.CertsRecordValue
	err := rv.Unmarshal(value)
	if err != nil {
		return nil
	}

	return &rv
}

