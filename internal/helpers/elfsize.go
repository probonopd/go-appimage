// Based on https://forum.golangbridge.org/t/calculate-the-size-of-an-elf/16064/5
// Author: Holloway, Chew Kean Ho <kean.ho.chew@zoralab.com>

package helpers

import (
	"debug/elf"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
)

// CalculateElfSize returns the size of an ELF binary as an int64 based on the information in the ELF header
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

// EmbedStringInSegment embeds a string in an ELF segment, returns error
func EmbedStringInSegment(path string, section string, s string) error {
	fmt.Println("")
	// Find Offset and Length of section
	uidata, err := GetSectionData(path, section)
	// PrintError("GetSectionData for '"+section+"'", err)
	if err != nil {
		os.Stderr.WriteString("Could not find section " + section + " in runtime, exiting\n")
		return (err)
	}
	fmt.Println("")
	fmt.Println("Section " + section + " before embedding:")
	fmt.Println(uidata)
	fmt.Println("")
	uioffset, uilength, err := GetSectionOffsetAndLength(path, section)
	PrintError("GetSectionData for '"+section+"'", err)
	if err != nil {
		os.Stderr.WriteString("Could not determine Offset and Length of " + section + " in runtime, exiting\n")
		return (err)
	}
	fmt.Println("Embedded "+section+" section Offset:", uioffset)
	fmt.Println("Embedded "+section+" section Length:", uilength)
	fmt.Println("")
	// Exit if data exceeds available space in section
	if len(s) > len(uidata) {
		os.Stderr.WriteString("does not fit into " + section + " section, exiting\n")
		return (err)
	}
	fmt.Println("Writing into "+section+" section...", uilength)
	// Seek file to ui_offset and write it there
	WriteStringIntoOtherFileAtOffset(s, path, uioffset)
	PrintError("GetSectionData for '"+section+"'", err)
	if err != nil {
		os.Stderr.WriteString("Could write into " + section + " section, exiting\n")
		return (err)
	}
	uidata, err = GetSectionData(path, section)
	PrintError("GetSectionData for '"+section+"'", err)
	if err != nil {
		os.Stderr.WriteString("Could not find section " + section + " in runtime, exiting\n")
		return (err)
	}
	fmt.Println("")
	fmt.Println("Embedded " + section + " section after embedding:")
	fmt.Println(uidata)
	fmt.Println("")
	fmt.Println("Embedded " + section + " section now contains:")
	fmt.Println(string(uidata))
	fmt.Println("")
	return (nil)
}
