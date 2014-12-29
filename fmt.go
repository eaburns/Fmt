// Fmt is a source code formatting harness for Acme.
// It is intended to replace Edit ,|myformatter for goimports and other formatters.
// Fmt must be used from within an Acme buffer or its tag.
// It takes a single argument: the formatting command to run over the buffer contents.
// Fmt provides two benefits over Edit ,|myformatter:
// 1) After formatting it doesn't leave you looking at the top of the buffer,
// but tries to show you where you were when you clicked Fmt.
// 2) If the formatter returns in error the buffer contents are left unchanged.
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"

	"code.google.com/p/goplan9/plan9/acme"
)

type bodyReader struct{ *acme.Win }

func (r bodyReader) Read(data []byte) (int, error) {
	return r.Win.Read("body", data)
}

type sizeReader struct {
	size int
	r    io.Reader
}

func (r *sizeReader) Read(data []byte) (int, error) {
	n, err := r.r.Read(data)
	r.size += n
	return n, err
}

type sizeWriter struct {
	size int
	w    io.Writer
}

func (w *sizeWriter) Write(data []byte) (int, error) {
	n, err := w.w.Write(data)
	w.size += n
	return n, err
}

type dataWriter struct{ *acme.Win }

func (w dataWriter) Write(data []byte) (int, error) {
	return w.Win.Write("data", data)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: Fmt <cmd>\n")
		os.Exit(1)
	}
	win, err := openWin()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open win: %s\n", err)
		os.Exit(1)
	}
	q0, q1, err := readAddr(win)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get the current selection: %s\n", err)
		os.Exit(1)
	}
	ffile, noChange, err := format(win, os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "format failed: %s\n", err)
		exit(1, ffile)
	}
	if !noChange {
		writeFile(win, ffile, q0, q1)
		exit(0, ffile)
	}
	noChange, err = equalsBody(win, ffile)
	if err != nil {
		exit(1, ffile)
	}
	if noChange {
		exit(0, ffile)
	}
	writeFile(win, ffile, q0, q1)
}

func writeFile(win *acme.Win, ffile string, q0, q1 int) {
	if err := writeBody(win, ffile); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write the body: %s\n", err)
		exit(1, ffile)
	}
	if err := showAddr(win, q0, q1); err != nil {
		fmt.Fprintf(os.Stderr, "failed to restore the selection: %s\n", err)
		exit(1, ffile)
	}
}

func exit(status int, ffile string) {
	if err := os.Remove(ffile); err != nil {
		fmt.Fprintf(os.Stderr, "failed to remove tempfile %s: %s\n", ffile, err)
	}
	os.Exit(status)
}

func openWin() (*acme.Win, error) {
	id, err := strconv.Atoi(os.Getenv("winid"))
	if err != nil {
		return nil, err
	}
	return acme.Open(id, nil)
}

func readAddr(win *acme.Win) (q0, q1 int, err error) {
	// This first read is bogus.
	// Acme zeroes the win's address the first time addr is opened.
	// So, we need to open it before setting addr=dot,
	// lest we just read back a zero address.
	if _, _, err := win.ReadAddr(); err != nil {
		return 0, 0, err
	}
	if err := win.Ctl("addr=dot\n"); err != nil {
		return 0, 0, err
	}
	return win.ReadAddr()
}

func showAddr(win *acme.Win, q0, q1 int) error {
	if err := win.Addr("#%d,#%d", q0, q1); err != nil {
		return err
	}
	return win.Ctl("dot=addr\nshow\n")
}

// If tmpFile is non-empty, it is created and must be removed by the caller.
func format(win *acme.Win, run []string) (tmpFile string, noChange bool, err error) {
	tf, err := ioutil.TempFile(os.TempDir(), "Fmt")
	if err != nil {
		return "", false, err
	}
	tmpFile = tf.Name()
	br := &sizeReader{0, bodyReader{win}}
	fw := &sizeWriter{0, tf}
	cmd := exec.Command(run[0], run[1:]...)
	cmd.Stdin = br
	cmd.Stdout = fw
	cmd.Stderr = os.Stderr
	if err = cmd.Run(); err != nil {
		tf.Close()
	} else {
		err = tf.Close()
	}
	noChange = fw.size == br.size
	return tmpFile, noChange, err
}

func writeBody(win *acme.Win, ffile string) error {
	tf, err := os.Open(ffile)
	if err != nil {
		return err
	}
	defer tf.Close()
	if err := win.Addr("0,$"); err != nil {
		return err
	}
	_, err = io.Copy(dataWriter{win}, tf)
	return err
}

func equalsBody(win *acme.Win, ffile string) (bool, error) {
	tf, err := os.Open(ffile)
	if err != nil {
		return false, err
	}
	defer tf.Close()
	win.Seek("body", 0, 0)
	fr := bufio.NewReader(tf)
	br := bufio.NewReader(&bodyReader{win})
	for {
		fbuf := make([]byte, 1)
		bbuf := make([]byte, 1)
		_, errf := fr.Read(fbuf)
		if errf != nil && errf != io.EOF {
			return false, errf
		}
		_, errb := br.Read(bbuf)
		if errb != nil && errb != io.EOF {
			return false, errb
		}
		if !bytes.Equal(fbuf, bbuf) {
			return false, nil
		}
		if errf == io.EOF && errb == io.EOF {
			break
		}
	}
	return true, nil
}
