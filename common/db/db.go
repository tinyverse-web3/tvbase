package db

import (
	ds "github.com/ipfs/go-datastore"
	badgerds "github.com/ipfs/go-ds-badger2"
	levelds "github.com/ipfs/go-ds-leveldb"
	ldbopts "github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/tinyverse-web3/tvbase/common/define"
)

type Datastore interface {
	ds.Batching // must be thread-safe
}

func CreateDataStore(dbRootDir string, mode define.NodeMode) (Datastore, error) {
	switch mode {
	case define.LightMode:
		return CreateBadgerDB(dbRootDir)
	case define.ServiceMode:
		return CreateLevelDB(dbRootDir)
	}
	return nil, nil
}

func CreateLevelDB(dbRootDir string) (*levelds.Datastore, error) {
	return levelds.NewDatastore(dbRootDir, &levelds.Options{
		Compression: ldbopts.NoCompression,
	})
}

func CreateBadgerDB(dbRootDir string) (*badgerds.Datastore, error) {
	defopts := badgerds.DefaultOptions
	defopts.SyncWrites = false
	defopts.Truncate = true
	return badgerds.NewDatastore(dbRootDir, &defopts)
}
