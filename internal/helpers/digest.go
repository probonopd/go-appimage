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
	fmt.Println("...hashing from offset", strconv.FormatInt(offset, 10), "for", strconv.FormatInt(length, 10), "bytes")
	s := io.NewSectionReader(f, offset, length)
	if _, err := io.Copy(h, s); err != nil {
		log.Fatal(err)
	}
}

func hashDummyRange(h hash.Hash, length int) {
	fmt.Println("...hashing for", strconv.Itoa(length), "bytes as if they were 0x00")
	h.Write(bytes.Repeat([]byte{0x00}, length))
}
