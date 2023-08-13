package db

import (
	"os"
	"path/filepath"

	ds "github.com/ipfs/go-datastore"
	badgerds "github.com/ipfs/go-ds-badger2"
	levelds "github.com/ipfs/go-ds-leveldb"
	ldbopts "github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/tinyverse-web3/tvbase/common/define"
	tvbaseLog "github.com/tinyverse-web3/tvbase/common/log"
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
	fullPath := dbRootDir
	if !filepath.IsAbs(fullPath) {
		rootPath, err := os.Getwd()
		if err != nil {
			tvbaseLog.Logger.Errorf("createLevelDB: error: %v", err)
			return nil, err
		}
		fullPath = filepath.Join(rootPath, fullPath)
	}
	return levelds.NewDatastore(fullPath, &levelds.Options{
		Compression: ldbopts.NoCompression,
	})
}

func CreateBadgerDB(dbRootDir string) (*badgerds.Datastore, error) {
	fullPath := dbRootDir
	if !filepath.IsAbs(fullPath) {
		rootPath, err := os.Getwd()
		if err != nil {
			tvbaseLog.Logger.Errorf("createBadgerDB: error: %v", err)
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