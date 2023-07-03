package dkvs

import (
	"bytes"

	"github.com/libp2p/go-libp2p/core/crypto"
	pb "github.com/tinyverse-web3/tvbase/dkvs/pb"
)

// 证书结构：dkvs.pb.Cert
// 证书由发行者记录在用户公钥下面，一般格式（路径可以自定义）
// addr: /服务名称/用户公钥/cert
// value: CertsRecordValue

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
		Name:         name,
		Type:         uint32(pb.CertType_Default),
		UserPubkey:   []byte(""),
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
		Name:         name,
		Type:         uint32(pb.CertType_Default),
		UserPubkey:   []byte(""),
		Data:         []byte(name),
		IssueTime:    issueTime,
		Ttl:          ttl,
		IssuerPubkey: gunPubkey,
		IssuerSign:   gunSign,
	}

	return EncodeCertsRecordValueWithCert(&cert, userData)
}

// decode from value by a GUN record
func DecodeGunValue(value []byte) *pb.Cert {
	return DecodeCert(value)
}

// generate value for a GUN record
func EncodeGunRecordValue(cv []*pb.Cert, userData []byte) []byte {
	rv := pb.CertsRecordValue{
		UserData: userData,
		CertVect: cv,
	}

	b, err := rv.Marshal()
	if err != nil {
		return nil
	}

	return b
}

// generate value for a GUN record
func EncodeGunRecordValueWithCert(cert *pb.Cert, userData []byte) []byte {
	rv := pb.CertsRecordValue{
		UserData: userData,
		CertVect: []*pb.Cert{cert},
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

func VerifyGunRecordValue(value []byte, issuetime uint64, ttl uint64) bool {

	rv := DecodeGunRecordValue(value)
	if rv == nil {
		return false
	}

	cert := FindGunCert(rv.CertVect)
	if cert == nil {
		return false
	}

	return cert.Ttl == ttl && cert.IssueTime == issuetime
}

// used to sign with private key
func IssueCert(data []byte, issuePubkey []byte) *pb.Cert {

	cert := pb.Cert{
		Version:      1,
		Name:         "",
		Type:         uint32(pb.CertType_Default),
		UserPubkey:   []byte(""),
		Data:         data,
		IssueTime:    TimeNow(),
		Ttl:          0, // for ever
		IssuerPubkey: issuePubkey,
		IssuerSign:   nil,
	}

	return &cert
}

// generate value for a certs record
func EncodeCertValue(name string, issueTime uint64, ttl uint64, issuePubkey []byte, issueSign []byte, userData []byte) []byte {
	cert := pb.Cert{
		Version:      1,
		UserPubkey:   []byte(""),
		Data:         []byte(name),
		IssueTime:    issueTime,
		Ttl:          ttl,
		IssuerPubkey: issuePubkey,
		IssuerSign:   issueSign,
	}

	return EncodeGunRecordValueWithCert(&cert, userData)
}

// generate value for a certs record
func EncodeCertsRecordValue(cv []*pb.Cert, userData []byte) []byte {
	rv := pb.CertsRecordValue{
		UserData: userData,
		CertVect: cv,
	}

	b, err := rv.Marshal()
	if err != nil {
		return nil
	}

	return b
}

// generate value for a certs record
func EncodeCertsRecordValueWithCert(cert *pb.Cert, userData []byte) []byte {
	rv := pb.CertsRecordValue{
		UserData: userData,
		CertVect: []*pb.Cert{cert},
	}

	b, err := rv.Marshal()
	if err != nil {
		return nil
	}

	return b
}

// decode from value by a GUN record
func DecodeCertsRecordValue(value []byte) *pb.CertsRecordValue {
	var rv pb.CertsRecordValue
	err := rv.Unmarshal(value)
	if err != nil {
		return nil
	}

	return &rv
}
