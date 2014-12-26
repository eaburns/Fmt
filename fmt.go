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
	status := 0
	ffile, err := format(win, os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "format failed: %s\n", err)
		status = 1
		goto out
	}
	if err := writeBody(win, ffile); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write the body: %s\n", err)
		status = 1
		goto out
	}
	if err := showAddr(win, q0, q1); err != nil {
		fmt.Fprintf(os.Stderr, "failed to restore the selection: %s\n", err)
		status = 1
		goto out
	}

out:
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
func format(win *acme.Win, run []string) (tmpFile string, err error) {
	tf, err := ioutil.TempFile(os.TempDir(), "Fmt")
	if err != nil {
		return "", err
	}
	tmpFile = tf.Name()
	cmd := exec.Command(run[0], run[1:]...)
	cmd.Stdin = bodyReader{win}
	cmd.Stdout = tf
	cmd.Stderr = os.Stderr
	if err = cmd.Run(); err != nil {
		tf.Close()
	} else {
		err = tf.Close()
	}
	return tmpFile, err
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