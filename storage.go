/* ratemilter is a milter service for postfix */
package main

import (
	"encoding/json"
	"os"
)

/* PersistentStorageFile is the name of a file to use
   as a persistant storage between service restarts */
const PersistentStorageFile = "/tmp/ratemilter.json"

/* SaveMemoryCache attempts to serialize and save contents
   of memory cache data structure to a persistent storage */
func SaveMemoryCache(MailboxMap *MailboxMemoryCache) error {
	// create and open a new file
	File, err := os.Create(PersistentStorageFile)
	if err != nil {
		return err
	}
	defer File.Close()
	// serialize and save data
	encoder := json.NewEncoder(File)
	// lock memory
	MailboxMap.Mutex.Lock()
	defer MailboxMap.Mutex.Unlock()
	// save data to storage
	if err := encoder.Encode(MailboxMap); err != nil {
		return err
	}
	return nil
}

/* LoadMemoryCache attempts to load and deserialize
   MemoryCache data from a persistent storage */
func LoadMemoryCache(MailboxMap *MailboxMemoryCache) error {
	// open persistent file
	File, err := os.Open(PersistentStorageFile)
	if err != nil {
		return err
	}
	defer File.Close()
	// lock memory
	MailboxMap.Mutex.Lock()
	defer MailboxMap.Mutex.Unlock()
	// deserialize data
	decoder := json.NewDecoder(File)
	if err := decoder.Decode(MailboxMap); err != nil {
		return err
	}
	return nil
}
