package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"

	"github.com/dgraph-io/badger/v3"
)

func main() {
	opts := badger.DefaultOptions(".zfs-inplace-recompress-resume")
	db, err := badger.Open(opts)
	if err != nil {
		log.Fatalf("Failed to open Badger resume database: %v", err)
	}

	err = filepath.WalkDir(".", func(fp string, fi os.DirEntry, err error) error {
		if err != nil {
			fmt.Println(err) // Can't walk here,
			return nil       // but continue walking elsewhere
		}

		if fi.Type().IsRegular() {
			// Find the file inode

			// Did we already handle this?
			db.View(func(txn *badger.Txn) error {
				txn.Get()
			})

			// Should
			var s unix.Stat_t
			err = unix.Lstat(fp, &s)
			if err != nil {

				return nil // skip
			}

			// Start a write transaction.
			/*			err = db.Update(func(txn *badger.Txn) error {
						// Set the key-value pair in the database.
						err := txn.Set(key, value)
						return err
					})*/
		}

		fmt.Println(fp) // print full path of the file
		return nil
	})

	if err != nil {
		fmt.Printf("error walking the folder: %v\n", err)
	}

	db.Close()

}
