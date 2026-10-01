// Package derivedcache stores disposable, integrity-checked projections.
// It owns no native source or player state; callers validate source identity.
package derivedcache

import (
	"bufio"
	"compress/zlib"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const HeaderSize = 88

type Format struct {
	Magic                 string
	PackedLimit, RawLimit int64
}

func (f Format) validate() error {
	if len(f.Magic) != 8 || f.PackedLimit <= 0 || f.RawLimit <= 0 || f.RawLimit >= 1<<62 {
		return fmt.Errorf("invalid derived cache format/budget")
	}
	return nil
}

func Load(path string, key [32]byte, format Format, decode func(io.Reader) error) (bool, error) {
	if err := format.validate(); err != nil {
		return false, err
	}
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() || info.Size() < HeaderSize || info.Size()-HeaderSize > format.PackedLimit {
		return false, fmt.Errorf("invalid derived cache size")
	}
	var header [HeaderSize]byte
	if _, err = io.ReadFull(f, header[:]); err != nil {
		return false, err
	}
	if string(header[:8]) != format.Magic || string(header[8:40]) != string(key[:]) {
		return false, fmt.Errorf("derived cache identity mismatch")
	}
	packed, raw := binary.LittleEndian.Uint64(header[72:80]), binary.LittleEndian.Uint64(header[80:88])
	if packed != uint64(info.Size()-HeaderSize) || raw == 0 || raw > uint64(format.RawLimit) {
		return false, fmt.Errorf("invalid derived cache lengths")
	}
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return false, err
	}
	if string(h.Sum(nil)) != string(header[40:72]) {
		return false, fmt.Errorf("derived cache checksum mismatch")
	}
	if _, err = f.Seek(HeaderSize, io.SeekStart); err != nil {
		return false, err
	}
	compressed := bufio.NewReaderSize(f, 64<<10)
	z, err := zlib.NewReader(compressed)
	if err != nil {
		return false, err
	}
	defer z.Close()
	r := &countedReader{Reader: io.LimitReader(z, int64(raw)+1)}
	if err = decode(r); err != nil {
		return false, err
	}
	trailing, err := io.Copy(io.Discard, r)
	if err != nil {
		return false, err
	}
	if trailing != 0 {
		return false, fmt.Errorf("derived cache trailing content")
	}
	if r.n != int64(raw) {
		return false, fmt.Errorf("derived cache expanded length mismatch")
	}
	if _, err := compressed.ReadByte(); err != io.EOF {
		return false, fmt.Errorf("derived cache trailing compressed content: %v", err)
	}
	return true, nil
}

type countedReader struct {
	io.Reader
	n int64
}

func (r *countedReader) Read(b []byte) (int, error) {
	n, e := r.Reader.Read(b)
	r.n += int64(n)
	return n, e
}

type boundedWriter struct {
	io.Writer
	n, limit int64
}

func (w *boundedWriter) Write(b []byte) (int, error) {
	if int64(len(b)) > w.limit-w.n {
		return 0, fmt.Errorf("derived cache size budget exceeded")
	}
	n, e := w.Writer.Write(b)
	w.n += int64(n)
	return n, e
}

// Publish a fully synced same-directory temporary file. Readers verify a held
// file before decoding; a competing rename or transient miss never publishes a
// partial catalog. Cache failure is an optimization failure, not a source error.
func Save(path string, key [32]byte, format Format, encode func(io.Writer) error) (err error) {
	if err = format.validate(); err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".pvfc-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer func() { f.Close(); os.Remove(tmp) }()
	if _, err = f.Write(make([]byte, HeaderSize)); err != nil {
		return err
	}
	h := sha256.New()
	packed := &boundedWriter{Writer: io.MultiWriter(f, h), limit: format.PackedLimit}
	z, err := zlib.NewWriterLevel(packed, zlib.BestSpeed)
	if err != nil {
		return err
	}
	raw := &boundedWriter{Writer: z, limit: format.RawLimit}
	if err = encode(raw); err != nil {
		z.Close()
		return err
	}
	if raw.n == 0 {
		z.Close()
		return fmt.Errorf("empty derived cache content")
	}
	if err = z.Close(); err != nil {
		return err
	}
	var header [HeaderSize]byte
	copy(header[:8], format.Magic)
	copy(header[8:40], key[:])
	copy(header[40:72], h.Sum(nil))
	binary.LittleEndian.PutUint64(header[72:80], uint64(packed.n))
	binary.LittleEndian.PutUint64(header[80:88], uint64(raw.n))
	if _, err = f.WriteAt(header[:], 0); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
