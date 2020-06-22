package kvdb

import (
	"context"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"os"
)

const (
	// DefaultTempDBFileName is the default name of the temporary bolt DB
	// file that we'll use to atomically compact the primary DB file on
	// startup.
	DefaultTempDBFileName = "temp-dont-use.db"

	// LastCompactionFileNameSuffix is the suffix we append to the file name
	// of a database file to record the timestamp when the last compaction
	// occurred.
	LastCompactionFileNameSuffix = ".last-compacted"
)

var (
	byteOrder = binary.BigEndian
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

	if TestBackend == LdbBackendName {
		db, err := GetLdbBackend(path, name)
		if err != nil {
			return nil, nil, err
		}
		return db, empty, nil
	} else if TestBackend == EtcdBackendName {
		etcdConfig, cancel, err := StartEtcdTestBackend(path, 0, 0)
		if err != nil {
			return nil, empty, err
		}
		backend, err := Open(
			EtcdBackendName, context.TODO(), etcdConfig,
		)
		return backend, cancel, err
	}

	return nil, nil, fmt.Errorf("unknown backend")
}
