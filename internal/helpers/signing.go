package helpers

import (
	"fmt"
	"os"
)

const (
	PubkeyFileName     = "pubkey.asc"            // Public key
	PrivkeyFileName    = "privkey.asc"           // Private key
	EncPrivkeyFileName = "privkey.asc.enc"       // Encrypted private key
	EnvSuperSecret     = "super_secret_password" // Name of the secret environment variable stored on Travis CI
)

func CalculateSHA256Digest(path string) string {
	// Calculate AppImage MD5 digest according to
	// https://github.com/AppImage/libappimage/blob/4d6f5f3d5b6c8c01c39b8ce0364b74cd6e4043c7/src/libappimage_shared/digest.c
	// The ELF sections
	// .digest_md5
	// .sha256_sig
	// .sig_key
	// need to be skipped, although I think
	// .upd_info
	// ought to be skipped, too
	fmt.Println("Calculating the sha256 digest...")
	var byteRangesToBeAssumedEmpty []ByteRange
	sectionsToBeSkipped := []string{".digest_md5", ".sha256_sig", ".sig_key"}
	for _, s := range sectionsToBeSkipped {
		offset, length, err := GetSectionOffsetAndLength(path, s)
		if err == nil {
			fmt.Println("Section", s, "offset", offset, "length", length)
			br := ByteRange{int64(offset), int64(length)}
			byteRangesToBeAssumedEmpty = append(byteRangesToBeAssumedEmpty, br)
		}
	}
	f, err := os.Open(path)
	if err != nil {
		PrintError("Cannot open file", err)
		os.Exit(1)
	}
	defer f.Close()
	h := CalculateDigestSkippingRanges(f, byteRangesToBeAssumedEmpty)
	return fmt.Sprintf("%x", h.Sum(nil))
}
