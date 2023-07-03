package db

import (
	"os"
	"path/filepath"

	badgerds "github.com/ipfs/go-ds-badger2"
	levelds "github.com/ipfs/go-ds-leveldb"
	ldbopts "github.com/syndtr/goleveldb/leveldb/opt"
	tvConfig "github.com/tinyverse-web3/tvbase/common/config"
	tvLog "github.com/tinyverse-web3/tvbase/common/log"
	db "github.com/tinyverse-web3/tvutil/db"
)

func CreateDataStore(dbRootDir string, mode tvConfig.NodeMode) (db.Datastore, error) {
	switch mode {
	case tvConfig.LightMode:
		return createBadgerDB(dbRootDir)
	case tvConfig.ServiceMode:
		return createLevelDB(dbRootDir)
	}
	return nil, nil
}

func createLevelDB(dbRootDir string) (*levelds.Datastore, error) {
	fullPath := dbRootDir
	if !filepath.IsAbs(fullPath) {
		rootPath, err := os.Getwd()
		if err != nil {
			tvLog.Logger.Errorf("createLevelDB: error: %v", err)
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
			tvLog.Logger.Errorf("createBadgerDB: error: %v", err)
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
