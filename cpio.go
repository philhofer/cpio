// package cpio implements the cpio archive format
//
// The cpio archive format is an (ancient) archive format
// similar to tar. There are a couple header formats for cpio,
// but the cpio tool that ships with busybox only implements
// the 'newc' format, so that is what this package implements.
//
// The API for this package closely mirrors the archive/tar
// package in the standard library.
//
// One common contemporary use of the cpio format is
// for Linux initramfs images.
package cpio

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"
)

var (
	ErrTooShort     = errors.New("cpio: header too short")
	ErrBadMagic     = errors.New("cpio: bad header magic")
	ErrClosed       = errors.New("cpio: writer already closed")
	ErrWriteTooLong = errors.New("cpio: write too long")
)

// Header represents a cpio header.
//
// For documentation on the exact semantics of
// each header field, see man cpio(5).
//
// Unlike the tar format, the Header itself
// does not contain "link names" for symbolic links.
// Symbolic links store the link path as the file contents.
//
// Note that the "newc" cpio format does not support
// files larger than 4GB.
type Header struct {
	Name                 string // filename
	Size                 uint32 // file size
	Ino                  int
	Devmajor, Devminor   int
	Rdevmajor, Rdevminor int
	Mode                 os.FileMode
	Uid, Gid             int
	Nlink                int
	Modtime              time.Time
	sum                  uint32 // likely to be 0
}

const newcSize = (13 * 8) + 6

var newcMagic = []byte{'0', '7', '0', '7', '0', '1'}

func be(b []byte) int {
	return int(binary.BigEndian.Uint32(b))
}

func itobe(b []byte, i int) {
	binary.BigEndian.PutUint32(b, uint32(i))
}

func (h *Header) parse(r io.Reader) error {
	var buf [newcSize]byte

	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return err
	}

	_ = buf[:newcSize]
	if !bytes.Equal(buf[:6], newcMagic) {
		return ErrBadMagic
	}

	var bin [(newcSize - 6) / 2]byte
	_, err = hex.Decode(bin[:], buf[6:])
	if err != nil {
		return err
	}

	h.Ino = be(bin[0:])
	h.Mode = osmode(uint32(be(bin[4:])))
	h.Uid = be(bin[8:])
	h.Gid = be(bin[12:])
	h.Nlink = be(bin[16:])
	h.Modtime = time.Unix(int64(be(bin[20:])), 0)
	h.Size = uint32(be(bin[24:]))
	h.Devmajor = be(bin[28:])
	h.Devminor = be(bin[32:])
	h.Rdevmajor = be(bin[36:])
	h.Rdevminor = be(bin[40:])
	namesize := be(bin[44:])
	h.sum = uint32(be(bin[48:]))

	// XXX: this is probably a lot more than PATH_MAX
	if namesize > 1024 {
		return fmt.Errorf("cpio: file name length %d too large", namesize)
	}

	// need a NULL byte at the end, and then
	// 4-byte alignment of the rest;
	// the header itself is 2-byte misaligned...
	namebuf := make([]byte, (namesize+3+2)&^3 - 2)
	_, err = io.ReadFull(r, namebuf)
	if err != nil {
		return err
	}
	h.Name = string(namebuf[:namesize-1])
	return nil
}

func (h *Header) write(w io.Writer) error {
	var binbuf [(newcSize - 6) / 2]byte
	itobe(binbuf[:], h.Ino)
	itobe(binbuf[4:], int(unixmode(h.Mode)))
	itobe(binbuf[8:], h.Uid)
	itobe(binbuf[12:], h.Gid)
	itobe(binbuf[16:], h.Nlink)
	itobe(binbuf[20:], int(h.Modtime.Unix()))
	itobe(binbuf[24:], int(h.Size))
	itobe(binbuf[28:], h.Devmajor)
	itobe(binbuf[32:], h.Devminor)
	itobe(binbuf[36:], h.Rdevmajor)
	itobe(binbuf[40:], h.Rdevminor)
	itobe(binbuf[44:], len(h.Name)+1) // null-terminated
	itobe(binbuf[48:], int(h.sum))

	buf := make([]byte, (newcSize+len(h.Name)+1+3)&^3)
	copy(buf, newcMagic)
	hex.Encode(buf[len(newcMagic):], binbuf[:])
	copy(buf[newcSize:], h.Name)

	_, err := w.Write(buf)
	return err
}

// Reader reads cpio archives from an io.Reader
type Reader struct {
	r       io.Reader
	curfile io.LimitedReader
	curpad  int
}

// NewReader constructs a new Reader.
func NewReader(r io.Reader) *Reader {
	return &Reader{r: r, curfile: io.LimitedReader{R: r}}
}

// Read reads the contents of the current file.
// Read will return io.EOF when the current file's
// contents are exhausted, at which point the caller
// may call (*Reader).Next() to advance to the next
// file in the archive.
func (r *Reader) Read(p []byte) (int, error) {
	return r.curfile.Read(p)
}

const trailername = "TRAILER!!!"

// Next advances to the next file in the archive.
// If the current file has not been read completely,
// its contents are discarded before advancing to the
// next file.
func (r *Reader) Next() (*Header, error) {
	if leftover := r.curfile.N + int64(r.curpad); leftover > 0 {
		_, err := io.CopyN(ioutil.Discard, r.r, leftover)
		if err != nil {
			return nil, err
		}
	}
	r.curfile.N = 0
	r.curpad = 0
	h := new(Header)
	err := h.parse(r.r)
	if err != nil {
		return nil, err
	}
	if h.Name == trailername {
		return nil, io.EOF
	}
	r.curfile.N = int64(h.Size)
	r.curpad = int(((h.Size + 3) &^ 3) - h.Size)
	return h, nil
}

// Writer writes cpio archives.
type Writer struct {
	w       io.Writer
	fsize   int64
	needpad int
	zpad    [4]byte
	closed  bool
}

// NewWriter constructs a new Writer.
func NewWriter(w io.Writer) *Writer {
	return &Writer{w: w}
}

// Flush flushes any padding necessary at the end of the current file.
// An error is returned if the Writer is closed, or if there are bytes
// left to be written for the current file (as indicated in the Size
// field of its Header; see (*Writer).WriteHeader()).
func (w *Writer) Flush() error {
	if w.closed {
		return ErrClosed
	}
	if w.fsize > 0 {
		return fmt.Errorf("cpio: writer flushed with %d bytes remaining to write", w.fsize)
	}
	if w.needpad > 0 {
		i, err := w.w.Write(w.zpad[:w.needpad])
		w.needpad -= i
		if err != nil {
			return err
		}
	}
	return nil
}

// WriteHeader writes a header to the archive.
// Following a call to WriteHeader, the caller should write
// h.Size bytes to the Writer before any subsequent calls
// to WriteHeader, Flush, or Close.
func (w *Writer) WriteHeader(h *Header) error {
	if err := w.Flush(); err != nil {
		return err
	}
	if err := h.write(w.w); err != nil {
		return err
	}
	w.fsize = int64(h.Size)
	w.needpad = int(int64((h.Size+3)&^3) - w.fsize)
	return nil
}

// Write writes file contents. It should be
// called after a call to WriteHeader.
//
// An error will be returned if the call to Write
// would write too many bytes to the archive
// (as indicated by the previously-written Header).
func (w *Writer) Write(b []byte) (int, error) {
	if w.closed {
		return 0, ErrClosed
	}
	if int64(len(b)) > w.fsize {
		return 0, ErrWriteTooLong
	}
	i, err := w.w.Write(b)
	w.fsize -= int64(i)
	return i, err
}

// Close writes the trailing archive entry and any
// necessary padding. After a call to Close, future
// calls to Write, WriteHeader, and Flush will fail
// with an error.
func (w *Writer) Close() error {
	if w.closed {
		return nil
	}
	err := w.Flush()
	if err != nil {
		return err
	}
	err = (&Header{
		Name: trailername,
	}).write(w.w)
	if err != nil {
		return err
	}
	w.closed = true
	return nil
}
