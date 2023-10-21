package db

import (
	ds "github.com/ipfs/go-datastore"
	badgerds "github.com/ipfs/go-ds-badger2"
	levelds "github.com/ipfs/go-ds-leveldb"
	ldbopts "github.com/syndtr/goleveldb/leveldb/opt"
	tvConfig "github.com/tinyverse-web3/tvbase/common/config"
)

type Datastore interface {
	ds.Batching // must be thread-safe
}

func CreateDataStore(dbRootDir string, mode tvConfig.NodeMode) (Datastore, error) {
	switch mode {
	case tvConfig.LightMode:
		return createBadgerDB(dbRootDir)
	case tvConfig.ServiceMode:
		return createLevelDB(dbRootDir)
	}
	return nil, nil
}

func createLevelDB(dbRootDir string) (*levelds.Datastore, error) {
	return levelds.NewDatastore(dbRootDir, &levelds.Options{
		Compression: ldbopts.NoCompression,
	})
}

func createBadgerDB(dbRootDir string) (*badgerds.Datastore, error) {
	defopts := badgerds.DefaultOptions
	defopts.SyncWrites = false
	defopts.Truncate = true
	return badgerds.NewDatastore(dbRootDir, &defopts)
}
