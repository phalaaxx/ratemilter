package main

import (
	"github.com/phalaaxx/godb"
)

/* VerifyLocal checks if named mailbox exist in a local cdb database */
func VerifyLocal(name string) bool {
	var value *string
	err := godb.CdbLookup(
		LocalCdb,
		func(db *godb.CdbReader) (err error) {
			value, err = db.Get(name)
			return err
		},
	)
	if err == nil && value != nil && len(*value) != 0 {
		return true
	}
	return false
}
