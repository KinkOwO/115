package derivedcache

import (
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Family struct{ Prefix, Magic string }
type Retention struct {
	MaxBytes      int64
	KeepPerFamily int
	Grace         time.Duration
}
type PruneResult struct {
	Removed int
	Bytes   int64
	Errors  int
}

// Prune only recognizes exact owned names with matching embedded identity.
// It never recurses, follows symlinks, removes directories or touches foreign
// files. Current keys and recently created versions survive even over budget.
func Prune(dir string, families []Family, protected map[string]bool, policy Retention, now time.Time) (PruneResult, error) {
	var result PruneResult
	if dir == "" || dir == "-" {
		return result, nil
	}
	if policy.MaxBytes <= 0 || policy.KeepPerFamily < 1 || policy.Grace < 0 {
		return result, fmt.Errorf("invalid cache retention policy")
	}
	root, err := filepath.Abs(dir)
	if err != nil {
		return result, err
	}
	info, err := os.Lstat(root)
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return result, fmt.Errorf("cache retention requires a real directory")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return result, err
	}
	type entry struct {
		path, family string
		info         os.FileInfo
		protected    bool
		rank         int
	}
	var files []entry
	var total int64
	for _, d := range entries {
		for _, family := range families {
			name := d.Name()
			if !strings.HasPrefix(name, family.Prefix) || !strings.HasSuffix(name, ".pvfc") {
				continue
			}
			hexKey := strings.TrimSuffix(strings.TrimPrefix(name, family.Prefix), ".pvfc")
			key, e := hex.DecodeString(hexKey)
			if e != nil || len(key) != 32 || hexKey != strings.ToLower(hexKey) {
				continue
			}
			path := filepath.Join(root, name)
			fi, e := os.Lstat(path)
			if e != nil || !fi.Mode().IsRegular() || fi.Size() < HeaderSize {
				continue
			}
			f, e := os.Open(path)
			if e != nil {
				continue
			}
			var header [40]byte
			_, e = io.ReadFull(f, header[:])
			f.Close()
			if e != nil || string(header[:8]) != family.Magic || string(header[8:]) != string(key) {
				continue
			}
			active := protected[filepath.Clean(path)] || protected[filepath.Join(dir, name)]
			files = append(files, entry{path, family.Prefix, fi, active, 0})
			total += fi.Size()
			break
		}
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].info.ModTime().Equal(files[j].info.ModTime()) {
			return files[i].path < files[j].path
		}
		return files[i].info.ModTime().After(files[j].info.ModTime())
	})
	ranks := map[string]int{}
	for i := range files {
		ranks[files[i].family]++
		files[i].rank = ranks[files[i].family]
	}
	for i := len(files) - 1; i >= 0; i-- {
		f := files[i]
		if f.protected || now.Sub(f.info.ModTime()) < policy.Grace || f.rank <= policy.KeepPerFamily && total <= policy.MaxBytes {
			continue
		}
		// Keep at least the newest file of each family even when the budget is
		// smaller than a single metadata file; caches are best-effort artifacts.
		if f.rank == 1 {
			continue
		}
		current, e := os.Lstat(f.path)
		if e != nil {
			result.Errors++
			continue
		}
		if !current.Mode().IsRegular() || !os.SameFile(f.info, current) || current.Size() != f.info.Size() || !current.ModTime().Equal(f.info.ModTime()) {
			continue
		}
		if e = os.Remove(f.path); e != nil {
			result.Errors++
			continue
		}
		result.Removed++
		result.Bytes += f.info.Size()
		total -= f.info.Size()
	}
	return result, nil
}
