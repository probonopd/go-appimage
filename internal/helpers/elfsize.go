// Based on https://forum.golangbridge.org/t/calculate-the-size-of-an-elf/16064/5
// Author: Holloway, Chew Kean Ho <kean.ho.chew@zoralab.com>

package helpers

import (
	"debug/elf"
	"encoding/binary"
	"io"
	"log"
	"os"
)

func CalculateElfSize(file string) int64 {

	// Open given elf file

	f, err := os.Open(file)
	PrintError("ioReader", err)
	// defer f.Close()
	if err != nil {
		return 0
	}

	_, err = f.Stat()
	PrintError("ioReader", err)
	if err != nil {
		return 0
	}

	e, err := elf.NewFile(f)
	if err != nil {
		PrintError("elfsize elf.NewFile", err)
		return 0
	}

	// Read identifier
	var ident [16]uint8
	_, err = f.ReadAt(ident[0:], 0)
	if err != nil {
		PrintError("elfsize read identifier", err)
		return 0
	}

	// Decode identifier
	if ident[0] != '\x7f' ||
		ident[1] != 'E' ||
		ident[2] != 'L' ||
		ident[3] != 'F' {
		log.Printf("Bad magic number at %d\n", ident[0:4])
		return 0
	}

	// Process by architecture
	sr := io.NewSectionReader(f, 0, 1<<63-1)
	var shoff, shentsize, shnum int64
	switch e.Class.String() {
	case "ELFCLASS64":
		hdr := new(elf.Header64)
		_, err = sr.Seek(0, 0)
		if err != nil {
			PrintError("elfsize", err)
			return 0
		}
		err = binary.Read(sr, e.ByteOrder, hdr)
		if err != nil {
			PrintError("elfsize", err)
			return 0
		}

		shoff = int64(hdr.Shoff)
		shnum = int64(hdr.Shnum)
		shentsize = int64(hdr.Shentsize)
	case "ELFCLASS32":
		hdr := new(elf.Header32)
		_, err = sr.Seek(0, 0)
		if err != nil {
			PrintError("elfsize", err)
			return 0
		}
		err = binary.Read(sr, e.ByteOrder, hdr)
		if err != nil {
			PrintError("elfsize", err)
			return 0
		}

		shoff = int64(hdr.Shoff)
		shnum = int64(hdr.Shnum)
		shentsize = int64(hdr.Shentsize)
	default:
		log.Println("unsupported elf architecture")
		return 0
	}

	// Calculate ELF size
	elfsize := shoff + (shentsize * shnum)
	// log.Println("elfsize:", elfsize, file)
	return elfsize
}
