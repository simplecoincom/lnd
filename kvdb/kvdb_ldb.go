package kvdb

import (
	"os"
	"path/filepath"

	_ "github.com/btcsuite/btcwallet/walletdb/ldb" // Import to register backend.
)

// GetLdbBackend opens (or creates if doesn't exits) a goleveldb
// backed database and returns a kvdb.Backend wrapping it.
func GetLdbBackend(path, name string) (Backend, error) {
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

		db, err = Create(LdbBackendName, dbFilePath)
	} else {
		db, err = Open(LdbBackendName, dbFilePath)
	}

	if err != nil {
		return nil, err
	}

	return db, nil
}
