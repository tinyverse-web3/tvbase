package dkvs

import (
	"bytes"
	"errors"

	"github.com/gogo/protobuf/proto"
	record "github.com/libp2p/go-libp2p-record"
	pb "github.com/tinyverse-web3/tvbase/dkvs/pb"
)

const _ValueType_Transfer = 0xffffffff

var _ record.Validator = Validator{}

// RecordKey returns the libp2p record key for a given peer ID.
func RecordKey(key string) string {
	//Comment out do not use /dkvs need to check
	return DefaultKeyPrefix + key
}

// Validator is an Dkvs record validator that satisfies the libp2p record
// validator interface.
type Validator struct {
}

// Validate validates an dkvs record.
func (v Validator) Validate(fullkey string, value []byte) error {
	//Is it necessary to verify the namespace, this needs to be discussed
	ns, key, err := record.SplitKey(fullkey)
	if err != nil || ns != DKVS_NAMESPACE {
		return ErrInvalidPath
	}

	// Parse the value into an IpnsEntry
	entry := new(pb.DkvsRecord)
	err = proto.Unmarshal(value, entry)
	if err != nil {
		return ErrBadRecord
	}

	return ValidateRecord("/"+key, entry)
}

// Select selects the best record by checking which has the highest sequence
// number and latest EOL.
//
// This function returns an error if any of the records fail to parse. Validate
// your records first!
func (v Validator) Select(fullkey string, vals [][]byte) (int, error) {
	ns, key, err := record.SplitKey(fullkey)
	if err != nil || ns != DKVS_NAMESPACE {
		return -1, ErrInvalidPath
	}

	vp, err := verifyPubKey("/"+key, vals[0], vals[1])
	if vp != 0 {
		Logger.Error(err)
		return vp, err
	}
	var recs []*pb.DkvsRecord
	for _, v := range vals {
		e := new(pb.DkvsRecord)
		if err := proto.Unmarshal(v, e); err != nil {
			return -1, err
		}
		recs = append(recs, e)
	}
	return selectRecord(recs, vals)
}

func selectRecord(recs []*pb.DkvsRecord, vals [][]byte) (int, error) {
	switch len(recs) {
	case 0:
		Logger.Error("no usable records in given set")
		return -1, errors.New("no usable records in given set")
	case 1:
		return 0, nil
	}

	var i int
	for j := 1; j < len(recs); j++ {
		cmp, err := Compare(recs[i], recs[j])
		if err != nil {
			Logger.Error(err)
			return -1, err
		}
		if cmp == 0 {
			cmp = bytes.Compare(vals[i], vals[j])
		}
		if cmp < 0 {
			i = j
		}
	}
	return i, nil
}

// Verify that the public key of the new record is the same as the old public key to prevent the record from being overwritten
func verifyPubKey(key string, newVal []byte, oldVal []byte) (int, error) {
	newRecord := new(pb.DkvsRecord)
	if err := proto.Unmarshal(newVal, newRecord); err != nil {
		return -1, err
	}
	oldRecord := new(pb.DkvsRecord)
	if err := proto.Unmarshal(oldVal, oldRecord); err != nil {
		return -1, err
	}

	switch oldRecord.ValidityType {
	case pb.DkvsRecord_EOL:
		if newRecord.ValueType == _ValueType_Transfer {
			// 处于转移状态的key，需要检查是否有权限接收该key
			var cert *pb.Cert = nil
			if newRecord.Data != nil {
				var c pb.Cert
				err := c.Unmarshal(newRecord.Data)
				if err != nil {
					Logger.Error(err)
					return -1, err
				}
				cert = &c
			}
			if !VerifyCertTransferConfirm(key, oldRecord.Value, cert, oldRecord.PubKey, newRecord.PubKey) {
				Logger.Error(ErrSignature)
				return -1, ErrPublicKeyMismatch
			}
		} else {
			// 正常状态，只检查是否是公钥相同
			cmp := bytes.Compare(newRecord.GetPubKey(), oldRecord.GetPubKey())
			if cmp != 0 {
				Logger.Error(ErrDifferentPublicKey)
				return -1, ErrDifferentPublicKey
			}
		}

	default:
	}

	return 0, nil
}
