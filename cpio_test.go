package cpio

import (
	"bytes"
	"io"
	"path/filepath"
	"io/ioutil"
	"os"
	"testing"
)

func filecmp(t *testing.T, h *Header, body []byte) {
	f, err := os.Open(filepath.Join("testdata", h.Name))
	if err != nil {
		t.Fatal(err)
	}
	fi, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != h.Mode.Perm() {
		t.Errorf("mode %o != %o", fi.Mode().Perm(), h.Mode.Perm())
	}
	if uint32(fi.ModTime().Unix()) != uint32(h.Modtime.Unix()) {
		t.Errorf("modtime %s != %s", fi.ModTime(), h.Modtime)
	}
	if fi.Size() != int64(h.Size) {
		t.Errorf("size %d != %d", fi.Size(), h.Size)
	}
	if len(body) != int(h.Size) {
		t.Errorf("body is %d bytes but header size is %d", len(body), h.Size)
	}
	realbody, err := ioutil.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(realbody, body) {
		t.Error("file contents not identical")
		t.Errorf("real file:\n%s", realbody)
		t.Errorf("cpio file:\n%s", body)
	}
}

func TestCpio(t *testing.T) {
	wantcpio, err := ioutil.ReadFile("testdata/dir.cpio")
	if err != nil {
		t.Fatal(err)
	}

	wantfiles, err := filepath.Glob("testdata/dir/*")
	if err != nil {
		t.Fatal(err)
	}

	r := NewReader(bytes.NewReader(wantcpio))
	var headers []*Header
	var bodies [][]byte
	for {
		h, err := r.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatal(err)
		}
		headers = append(headers, h)
		body, err := ioutil.ReadAll(r)
		if err != nil {
			t.Fatal(err)
		}
		bodies = append(bodies, body)
		filecmp(t, h, body)
	}
	if len(headers) != len(wantfiles) {
		t.Fatal("didn't get all the files...?")
	}

	var buf bytes.Buffer
	w := NewWriter(&buf)
	for i, h := range headers {
		err := w.WriteHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		_, err = w.Write(bodies[i])
		if err != nil {
			t.Fatal(err)
		}
	}
	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Ideally, we'd check for the output to be bit-identical,
	// but the fact that the newc encoding uses hex means that
	// a single cpio archive has many valid encodings. Check
	// that the output is "the same" by reading the headers
	// back and checking their contents.
	if len(buf.Bytes()) != len(wantcpio) {
		t.Error("didn't produce identical-length archive")
	}

	nr := NewReader(&buf)
	checked := 0
	for {
		h, err := nr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatal(err)
		}
		if *h != *headers[checked] {
			t.Errorf("header for %q not the same after encoding", h.Name)
			t.Errorf("%+v", h)
			t.Errorf("%+v", headers[checked])
		}
		checked++
	}
	if checked != len(headers) {
		t.Errorf("missing headers? checked %d", checked)
	}
}
