package hashfile

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

// SHA256File returns lowercase hex SHA-256 and size of the file at path.
func SHA256File(path string) (hash string, size int64, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}
