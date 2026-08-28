package store

import "os"

// FileSize, veritabani dosyasinin (WAL/SHM dahil) toplam boyutunu dondurur.
func FileSize(path string) int64 {
	total := int64(0)
	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		if fi, err := os.Stat(p); err == nil {
			total += fi.Size()
		}
	}
	return total
}
