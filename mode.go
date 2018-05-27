package cpio

import (
	"os"
)

// Ordinary we'd just grab these from package syscall,
// but we need these to be defined on non-Unix platforms.
//
// see inode(7) for definitions
const (
	s_IFMT   = 0170000
	s_IFSOCK = 0140000
	s_IFLNK  = 0120000
	s_IFREG  = 0100000
	s_IFBLK  = 0060000
	s_IFDIR  = 0040000
	s_IFCHR  = 0020000
	s_IFIFO  = 0010000
	s_ISUID  = 0004000
	s_ISGID  = 0002000
	s_ISVTX  = 0001000
)

func osmode(u uint32) os.FileMode {
	mode := os.FileMode(u) & os.ModePerm
	switch u&s_IFMT {
	case s_IFBLK:
		mode |= os.ModeDevice
	case s_IFCHR:
		mode |= os.ModeDevice | os.ModeCharDevice
	case s_IFDIR:
		mode |= os.ModeDir
	case s_IFIFO:
		mode |= os.ModeNamedPipe
	case s_IFLNK:
		mode |= os.ModeSymlink
	case s_IFSOCK:
		mode |= os.ModeSocket
	}
	if u&s_ISGID != 0 {
		mode |= os.ModeSetgid
	}
	if u&s_ISUID != 0 {
		mode |= os.ModeSetuid
	}
	if u&s_ISVTX != 0 {
		mode |= os.ModeSticky
	}
	return mode
}

func unixmode(mode os.FileMode) uint32 {
	out := uint32(mode&os.ModePerm)
	switch mode&os.ModeType {
	case os.ModeDir:
		out |= s_IFDIR
	case os.ModeSymlink:
		out |= s_IFLNK
	case os.ModeNamedPipe:
		out |= s_IFIFO
	case os.ModeSocket:
		out |= s_IFSOCK
	case os.ModeDevice:
		if mode&os.ModeCharDevice != 0 {
			out |= s_IFCHR
		} else {
			out |= s_IFBLK
		}
	default:
		out |= s_IFREG
	}
	if mode&os.ModeSetuid != 0 {
		out |= s_ISUID
	}
	if mode&os.ModeSetgid != 0 {
		out |= s_ISGID
	}
	if mode&os.ModeSticky != 0 {
		out |= s_ISVTX
	}
	return out
}
