package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/dgraph-io/badger/v3"
	"github.com/spf13/pflag"
)

var totalfiles, totalbytes atomic.Uint64
var skipfiles, skipbytes atomic.Uint64

var minfilesize *int64
var debugflag, noresume *bool
var skipratio *float64
var ignorelist = []string{
	// Compressed images
	"jpg",
	"jpeg",
	"png",
	"gif",
	"webp",
	// Compressed archive files
	"zip",
	"gz",
	"bz2",
	"xz",
	"7z",
	"z77",
	"rar",
	// Compressed video files
	"mp4",  //
	"avi",  //
	"mkv",  // matroska video
	"flv",  // flv video
	"webm", // webm video
	// Compressed audio files
	"mp3",
	"wav",
	"ogg",
	"flac",
	// Other
	"pdf",
	"doc",
	"docx",
	"xls",
	"xlsx",
	"ppt",
	"pptx",
	"odt",
	"ods",
	"odp",
	"odg",
	"odf",
	"odc",
	"odm",
	"odt",
	"ncf", // netcdf
	"deb", // debian package

}

func log(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

func debug(format string, args ...interface{}) {
	if *debugflag {
		log(format, args...)
	}
}

func processfile(fp string, fi os.DirEntry, db *badger.DB, buffer []byte) error {
	fileinfo, err := fi.Info()
	if err != nil {
		return err
	}

	if fileinfo.Size() <= *minfilesize {
		debug("Skipping too small file %s", fp)
		skipfiles.Add(1)
		skipbytes.Add(uint64(fileinfo.Size()))
		return nil
	}

	for _, suffix := range ignorelist {
		if strings.HasSuffix(strings.ToLower(fp), suffix) {
			// Skip
			debug("Skipping ignored file %s", fp)
			skipfiles.Add(1)
			skipbytes.Add(uint64(fileinfo.Size()))
			return nil
		}
	}

	sysstat, ok := fileinfo.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("unknown file type %T", fileinfo.Sys())
	}

	// See if the inode has been handled already
	var skip bool
	if db != nil {
		err = db.View(func(txn *badger.Txn) error {
			b := make([]byte, 8)
			binary.LittleEndian.PutUint64(b, uint64(sysstat.Ino))
			item, err := txn.Get(b)
			if err == nil {
				err = item.Value(func(val []byte) error {
					if string(val) == "handled" {
						debug("Skipping handled file %s", fp)
						skipfiles.Add(1)
						skipbytes.Add(uint64(fileinfo.Size()))
						skip = true
					}
					return nil
				})
			}
			if err == badger.ErrKeyNotFound {
				return nil
			}
			return err
		})
	}
	if err != nil {
		return err
	}
	if skip {
		return nil
	}

	if *skipratio != 0 && float64(sysstat.Blocks)*512*(*skipratio) < float64(fileinfo.Size()) { // If file is already compressed 1.2:1 then skip it
		// Already compressed or sparse, skip
		debug("Skipping already compressed or sparse file %s", fp)
		skipfiles.Add(1)
		skipbytes.Add(uint64(fileinfo.Size()))
		return nil
	}

	if fileinfo.Size() == 0 {
		debug("Skipping zero bytes file %s", fp)
		skipfiles.Add(1)
		skipbytes.Add(uint64(fileinfo.Size()))
		return nil
	}

	// Process the file
	debug("Processing file %s with size %v bytes (uses %v bytes)", fp, fileinfo.Size(), sysstat.Blocks*512)

	source, err := os.Open(fp)
	if err != nil {
		return err
	}
	target, err := os.OpenFile(fp, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	// Copy from source to target
	copied, err := io.CopyBuffer(target, source, buffer)
	if err != nil {
		return err
	}
	target.Close()
	source.Close()

	if copied != sysstat.Size {
		return fmt.Errorf("copied %d bytes instead of %d", copied, sysstat.Size)
	}

	// Set the last modified timestamp to the original
	err = os.Chtimes(fp, fileinfo.ModTime(), fileinfo.ModTime())
	if err != nil {
		return err
	}

	// Start a write transaction.
	if db != nil {
		err = db.Update(func(txn *badger.Txn) error {
			// Set the key-value pair in the database.
			b := make([]byte, 8)
			binary.LittleEndian.PutUint64(b, uint64(sysstat.Ino))
			err := txn.Set(b, []byte("handled"))
			return err
		})
	}

	totalfiles.Add(1)
	totalbytes.Add(uint64(fileinfo.Size()))

	return err
}

func main() {
	ignore := pflag.String("ignore", strings.Join(ignorelist, ","), "Ignore files with these extensions")
	debugflag = pflag.Bool("debug", false, "Debug mode")
	noresume = pflag.Bool("noresume", false, "Dont create or use the resume database")
	skipratio = pflag.Float64("skipratio", 1.5, "Skip files that are already compressed more than this ratio (1.5:1 default, 0 = dont skip)")
	minfilesize = pflag.Int64("minfilesize", 16384, "Minimum filesize to process")
	threads := pflag.Int32("threads", int32(runtime.NumCPU()*2), "Number of parallel file IO threads")
	buffersize := pflag.Int32("buffersize", 16*1024*1024, "Buffer size per thread for IO")
	pflag.Parse()

	ignorelist = []string{}
	for _, pattern := range strings.Split(*ignore, ",") {
		ignorelist = append(ignorelist, "."+strings.ToLower(pattern))
	}

	var db *badger.DB
	var err error

	if !*noresume {
		opts := badger.DefaultOptions(".zfs-inplace-recompress-resume")
		db, err = badger.Open(opts)
		if err != nil {
			log("Failed to open Badger resume database: %v", err)
			os.Exit(1)
		}
	}

	type queueItem struct {
		fp string
		fi os.DirEntry
	}

	filequeue := make(chan queueItem, *threads)

	var abort bool

	// Ctrl-C handler to set abort
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		<-c
		log("Terminating, please wait for threads to finish tasks ...")
		abort = true
	}()

	var globalerror bool
	var workers sync.WaitGroup
	for i := 0; i < runtime.NumCPU(); i++ {
		workers.Add(1)
		go func() {
			buffer := make([]byte, *buffersize)
			for item := range filequeue {
				err := processfile(item.fp, item.fi, db, buffer)
				if err != nil {
					log("Error processing file %s: %v", item.fp, err)
					globalerror = true
				}
			}
			workers.Done()
		}()
	}

	err = filepath.WalkDir(".", func(fp string, di os.DirEntry, err error) error {
		if globalerror {
			return errors.New("Aborted due to global error")
		}
		if abort {
			return errors.New("Aborted due to interrupt")
		}

		if err != nil {
			log("Error walking directory: %v", err)
			return nil // but continue walking elsewhere
		}

		if di.Type().IsRegular() {
			// Find the file inode
			filequeue <- queueItem{fp, di}
		}
		return nil
	})

	close(filequeue)
	workers.Wait()

	if db != nil {
		db.Close()
	}

	log("Processed %v files, %v bytes", totalfiles.Load(), totalbytes.Load())
	log("Skipped %v files, %v bytes", skipfiles.Load(), skipbytes.Load())

	if err != nil {
		log("Error walking directory: %v", err)
		os.Exit(1)
	} else {
		if !*noresume {
			os.RemoveAll(".zfs-inplace-recompress-resume")
		}
	}

}
