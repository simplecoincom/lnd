// +build !js !wasm

package kvdb

import (
	"os"
	"path/filepath"

	_ "github.com/btcsuite/btcwallet/walletdb/bdb" // Import to register backend.
)

// GetBoltBackend opens (or creates if doesn't exits) a bbolt
// backed database and returns a kvdb.Backend wrapping it.
func GetBoltBackend(path, name string, noFreeListSync bool) (Backend, error) {
	dbFilePath := filepath.Join(path, name)
	var (
		db  Backend
		err error
	)

	if !fileExists(dbFilePath) {
		if !fileExists(path) {
			if err := os.MkdirAll(path, 0700); err != nil {
				return nil, err
			}
		}

		db, err = Create(BoltBackendName, dbFilePath, noFreeListSync)
	} else {
		db, err = Open(BoltBackendName, dbFilePath, noFreeListSync)
	}

	if err != nil {
		return nil, err
	}

	return db, nil
}
