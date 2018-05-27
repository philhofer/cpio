package cpio

import (
	"testing"
)

func TestMode(t *testing.T) {
	modes := []uint32{
		s_IFREG,
		s_IFREG|s_ISUID,
		s_IFREG|s_ISVTX,
		s_IFLNK|s_ISGID,
		s_IFDIR,
		s_IFCHR,
		s_IFBLK,
		s_IFIFO|s_ISGID,
	}

	for _, m := range modes {
		out := unixmode(osmode(m))
		if out != m {
			t.Errorf("0%o (in) != 0%o (out)", m, out)
		}
	}
}
