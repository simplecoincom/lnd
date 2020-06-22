package kvdb

import (
	"fmt"
	"os"
)

// fileExists returns true if the file exists, and false otherwise.
func fileExists(path string) bool {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}

	return true
}

// GetTestBackend opens (or creates if doesn't exist) a bbolt or etcd
// backed database (for testing), and returns a kvdb.Backend and a cleanup
// func. Whether to create/open bbolt or embedded etcd database is based
// on the TestBackend constant which is conditionally compiled with build tag.
// The passed path is used to hold all db files, while the name is only used
// for bbolt.
func GetTestBackend(path, name string) (Backend, func(), error) {
	empty := func() {}
	if TestBackend == EtcdBackendName {
		return GetEtcdTestBackend(path, name)
	} else if TestBackend == LdbBackendName {
		db, err := GetLdbBackend(path, name)
		if err != nil {
			return nil, nil, err
		}
		return db, empty, nil
	}

	return nil, nil, fmt.Errorf("unknown backend")
}
