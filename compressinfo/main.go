package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

func log(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

type ratioinfo struct {
	size int64
	blk  int64
}

func main() {
	ratios := map[string][]ratioinfo{}
	inodes := map[uint64]struct{}{}
	err := filepath.WalkDir(".", func(fp string, fi os.DirEntry, err error) error {
		if err != nil {
			log("Error walking directory: %v", err)
			return nil // but continue walking elsewhere
		}

		if fi.Type().IsRegular() {
			fileinfo, err := fi.Info()
			if err != nil {
				return err
			}

			sysstat, ok := fileinfo.Sys().(*syscall.Stat_t)
			if !ok {
				return fmt.Errorf("unknown file type %T", fileinfo.Sys())
			}

			if _, found := inodes[sysstat.Ino]; found {
				// Already handled
				return nil
			}

			lastdot := strings.LastIndex(fp, ".")
			if lastdot == -1 {
				// file has no extension
				return nil
			}
			lastslash := strings.LastIndex(fp, "/")
			if lastdot < lastslash {
				// file has no extension
				return nil
			}

			suffix := fp[lastdot+1:]
			ratios[suffix] = append(ratios[suffix], ratioinfo{sysstat.Size, int64(sysstat.Blocks)})
			inodes[sysstat.Ino] = struct{}{}
		}
		return nil
	})
	if err != nil {
		log("Error walking directory: %v", err)
		os.Exit(1)
	}

	fmt.Println("Uncompressable file extensions")
	for suffix, ratios := range ratios {
		var totalbytes int64
		var totalblocks int64
		var count int
		for _, r := range ratios {
			if r.size < 16384 {
				continue
			}
			count++
			totalbytes += r.size
			totalblocks += r.blk
		}
		// No bytes, so we can't estimate
		if totalbytes == 0 {
			continue
		}
		// Statistically insignificant
		if count <= 10 {
			continue
		}
		// It compresses, we only want the uncompressable ones
		if totalblocks*512 < totalbytes {
			continue
		}
		// Show me the files
		fmt.Printf("%s: %v vs %v (%v files)\n", suffix, totalbytes, totalblocks*512, count)
	}
}
