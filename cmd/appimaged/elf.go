package main

// #include <elf.h>
// #include <byteswap.h>
// #include <stdio.h>
// #include <stdint.h>
// #include <errno.h>
// #include <stdlib.h>
// #include <unistd.h>
// #include <string.h>
// #include <fcntl.h>

import "C"

/*
Compile with:
gcc elfsize.c -o elfsize
Example:
ls -l						126584
Calculation using the values also reported by readelf -h:
Start of section headers	e_shoff		124728
Size of section headers		e_shentsize	64
Number of section headers	e_shnum		29
e_shoff + ( e_shentsize * e_shnum ) =		126584
*/

// typedef Elf32_Nhdr Elf_Nhdr;

// static char *fname;
// static Elf64_Ehdr ehdr;
// static Elf64_Phdr *phdr;

// #if __BYTE_ORDER == __LITTLE_ENDIAN
// #define ELFDATANATIVE ELFDATA2LSB
// #elif __BYTE_ORDER == __BIG_ENDIAN
// #define ELFDATANATIVE ELFDATA2MSB
// #else
// #error "Unknown machine endian"
// #endif

// int calculateOffset(int argc, char **argv)
// {
// 	ssize_t ret;
// 	int fd;

// 	if (argc != 2) {
// 		fprintf(stderr, "Usage: %s <ELF>\n", argv[0]);
// 		return 1;
// 	}
// 	fname = argv[1];

// 	long unsigned int size = get_elf_size(fname);
// 	fprintf(stderr, "Estimated ELF size on disk: %lu bytes \n", size);
// 	return size;
// }
