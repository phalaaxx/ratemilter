package main

import (
	cdb "github.com/phalaaxx/cdb"
)

/* VerifyLocal checks if named mailbox exist in a local cdb database */
func VerifyLocal(name string) bool {
	var value *string
	err := cdb.CdbLookup(
		LocalCdb,
		func(db *cdb.CdbReader) (err error) {
			value, err = db.Get(name)
			return err
		},
	)
	if err == nil && value != nil && len(*value) != 0 {
		return true
	}
	return false
}
