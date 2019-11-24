package helpers

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"hash"
	"io"
	"log"
	"os"
	"sort"
	"strconv"
)

type ByteRange struct {
	Offset int64
	Length int64
}

// CalculateDigestSkippingRanges calculates the sha256 hash of a file
// while assuming that the supplied byteRanges are consisting fo '0x00's
func CalculateDigestSkippingRanges(f *os.File, ranges []ByteRange) hash.Hash {

	fi, err := f.Stat()
	if err != nil {
		log.Fatal(err)
	}

	// fmt.Printf("The file is %d bytes long\n", fi.Size())

	// Sort the ranges by Offset, so that we can work on them one after another
	sort.Slice(ranges, func(i, j int) bool {
		return ranges[i].Offset < ranges[j].Offset
	})

	h := sha256.New()

	// Add to the hash the checksum for the area between Offset 0 and the first ByteRange
	hashRange(f, h, 0, ranges[0].Offset)

	// Add to the hash the checksum for each range to be excluded
	numberOfRanges := len(ranges)
	for i, byterange := range ranges {
		// fmt.Println("range Offset", byterange.Offset, "Length", byterange.Length)
		hashDummyRange(h, int(byterange.Length))
		// Add to the hash the checksum for the area after the excluded range until the end of the file
		if i == numberOfRanges-1 {
			// This was the last excluded range, so we continue to the end of the file
			hashRange(f, h, byterange.Offset+byterange.Length, fi.Size()-(byterange.Offset+byterange.Length))
		} else {
			// Up to the beginning of the next excluded range
			hashRange(f, h, byterange.Offset+byterange.Length, ranges[i+1].Offset-(byterange.Offset+byterange.Length))
		}
	}

	return h
}

func hashRange(f *os.File, h hash.Hash, offset int64, length int64) {
	if length == 0 {
		return
	}
	fmt.Println("...hashing", strconv.FormatInt(length, 10), "bytes")
	s := io.NewSectionReader(f, offset, length)
	if _, err := io.Copy(h, s); err != nil {
		log.Fatal(err)
	}
}

func hashDummyRange(h hash.Hash, length int) {
	if length == 0 {
		return
	}
	fmt.Println("...hashing", strconv.Itoa(length), "bytes as if they were 0x00")
	h.Write(bytes.Repeat([]byte{0x00}, length))
}

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

	// TheAssassin's implementation of the signature checking only zeros ".sha256_sig", ".sig_key"
	// according to him, and if we do the same then we get the same results as his tools
	sectionsToBeSkipped := []string{".sha256_sig", ".sig_key"} // Why not the non-spec-conforming ".digest_md5" ???
	for _, s := range sectionsToBeSkipped {
		offset, length, err := GetSectionOffsetAndLength(path, s)
		if err == nil {
			if length == 0 {
				continue
			}
			fmt.Println("Assuming section", s, "offset", offset, "length", length, "to contain only '0x00's")
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
