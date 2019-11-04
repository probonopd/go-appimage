package main

import (
	"log"
	"unsafe"
)

// Calculates the offset at which the squashfs filesystem starts in type-2 images
// which equals the size of the ELF executable prepended to the squashfs image.
// Currently it is C code (because I don't know how to write this in Go)
// but eventually may be rewritten to do things natively in Go. Any help appreciated.
// https://forum.golangbridge.org/t/calculate-the-size-of-an-elf/16064

/*
// https://github.com/draffensperger/go-interlang/blob/master/go_to_c/c_in_comment/main.go
// This preamble is interpreted as C code
// See cgo docs: https://golang.org/cmd/cgo/
#include <elf.h>
#include <byteswap.h>
#include <stdio.h>
#include <stdint.h>
#include <errno.h>
#include <stdlib.h>
#include <unistd.h>
#include <string.h>
#include <fcntl.h>

typedef Elf32_Nhdr Elf_Nhdr;

static char *fname;
static Elf64_Ehdr ehdr;
static Elf64_Phdr *phdr;

#if __BYTE_ORDER == __LITTLE_ENDIAN
#define ELFDATANATIVE ELFDATA2LSB
#elif __BYTE_ORDER == __BIG_ENDIAN
#define ELFDATANATIVE ELFDATA2MSB
#else
#error "Unknown machine endian"
#endif

static uint16_t file16_to_cpu(uint16_t val)
{
	if (ehdr.e_ident[EI_DATA] != ELFDATANATIVE)
		val = bswap_16(val);
	return val;
}

static uint32_t file32_to_cpu(uint32_t val)
{
	if (ehdr.e_ident[EI_DATA] != ELFDATANATIVE)
		val = bswap_32(val);
	return val;
}

static uint64_t file64_to_cpu(uint64_t val)
{
	if (ehdr.e_ident[EI_DATA] != ELFDATANATIVE)
		val = bswap_64(val);
	return val;
}

static long unsigned int read_elf32(int fd)
{
	Elf32_Ehdr ehdr32;
	ssize_t ret, i;

	ret = pread(fd, &ehdr32, sizeof(ehdr32), 0);
	if (ret < 0 || (size_t)ret != sizeof(ehdr)) {
		fprintf(stderr, "Read of ELF header from %s failed: %s\n",
			fname, strerror(errno));
	}

	ehdr.e_shoff		= file32_to_cpu(ehdr32.e_shoff);
	ehdr.e_shentsize	= file16_to_cpu(ehdr32.e_shentsize);
	ehdr.e_shnum		= file16_to_cpu(ehdr32.e_shnum);

	return(ehdr.e_shoff + (ehdr.e_shentsize * ehdr.e_shnum));
}

static long unsigned int read_elf64(int fd)
{
	Elf64_Ehdr ehdr64;
	ssize_t ret, i;

	ret = pread(fd, &ehdr64, sizeof(ehdr64), 0);
	if (ret < 0 || (size_t)ret != sizeof(ehdr)) {
		fprintf(stderr, "Read of ELF header from %s failed: %s\n",
			fname, strerror(errno));
	}

	ehdr.e_shoff		= file64_to_cpu(ehdr64.e_shoff);
	ehdr.e_shentsize	= file16_to_cpu(ehdr64.e_shentsize);
	ehdr.e_shnum		= file16_to_cpu(ehdr64.e_shnum);

	return(ehdr.e_shoff + (ehdr.e_shentsize * ehdr.e_shnum));
}

long unsigned int get_elf_size(char *fname)
{
	ssize_t ret;
	int fd;
	long unsigned int size = 0;

	fd = open(fname, O_RDONLY);
	if (fd < 0) {
		fprintf(stderr, "Cannot open %s: %s\n",
			fname, strerror(errno));
		return(1);
	}
	ret = pread(fd, ehdr.e_ident, EI_NIDENT, 0);
	if (ret != EI_NIDENT) {
		fprintf(stderr, "Read of e_ident from %s failed: %s\n",
			fname, strerror(errno));
		return(1);
	}
	if ((ehdr.e_ident[EI_DATA] != ELFDATA2LSB) &&
	    (ehdr.e_ident[EI_DATA] != ELFDATA2MSB))
	{
		fprintf(stderr, "Unkown ELF data order %u\n",
			ehdr.e_ident[EI_DATA]);
		return(1);
	}
	if(ehdr.e_ident[EI_CLASS] == ELFCLASS32) {
		size = read_elf32(fd);
	} else if(ehdr.e_ident[EI_CLASS] == ELFCLASS64) {
		size = read_elf64(fd);
	} else {
		fprintf(stderr, "Unknown ELF class %u\n", ehdr.e_ident[EI_CLASS]);
		return(1);
	}

	close(fd);
	return size;
}

int calc(char *fname)
{
	ssize_t ret;
	int fd;
    // fprintf(stderr, "File %s \n", fname);
	long unsigned int size = get_elf_size(fname);
	// fprintf(stderr, "Estimated ELF size on disk: %lu bytes \n", size);
	return size;
}
*/
import "C"

func calculateElfSize(filepath string) int64 {

	// C.CString uses C heap, we must free it
	var cstr *C.char = C.CString(filepath)
	// defer is convenient for clean-up
	defer C.free(unsafe.Pointer(cstr))
	C.puts(cstr)
	total := int64(C.calc(cstr))
	log.Println("elfsize: offset:", total, filepath)
	return total
}
