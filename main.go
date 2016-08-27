package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
)

func main() {
	err := run()
	if err != nil {
		panic(err)
	}
}

func run() error {
	cmd := exec.Command("scp", "-t", "/tmp")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	err = cmd.Start()
	if err != nil {
		return err
	}

	err = func() error {
		s, err := newSource(stdin, stdout)
		if err != nil {
			return err
		}
		defer s.close()

		mode := os.FileMode(0644)
		filename := "test1"
		content := "content1\n"
		err = s.writeFile(mode, int64(len(content)), filename, bytes.NewBufferString(content))
		if err != nil {
			return err
		}

		mode = os.FileMode(0406)
		filename = "test2"
		content = "content2\n"
		err = s.writeFile(mode, int64(len(content)), filename, bytes.NewBufferString(content))
		if err != nil {
			return err
		}

		return nil
	}()
	if err != nil {
		return err
	}

	err = cmd.Wait()
	if err != nil {
		return err
	}
	return nil
}

const (
	ok         = '\x00'
	warning    = '\x01'
	fatalError = '\x02'
)

type source struct {
	remIn     io.WriteCloser
	remOut    io.Reader
	remReader *bufio.Reader
}

func newSource(remIn io.WriteCloser, remOut io.Reader) (*source, error) {
	s := &source{
		remIn:     remIn,
		remOut:    remOut,
		remReader: bufio.NewReader(remOut),
	}

	b, msg, err := s.readReply()
	fmt.Printf("firstReply b=%v, msg=%s\n", b, msg)
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (s *source) close() error {
	return s.remIn.Close()
}

func (s *source) writeFile(mode os.FileMode, size int64, name string, body io.Reader) error {
	_, err := fmt.Fprintf(s.remIn, "C%#4o %d %s\n", mode, size, name)
	if err != nil {
		return fmt.Errorf("failed to write scp file header: err=%s", err)
	}
	_, err = io.Copy(s.remIn, body)
	if err != nil {
		return fmt.Errorf("failed to write scp file body: err=%s", err)
	}
	b, msg, err := s.readReply()
	fmt.Printf("reply after writing body. filename=%s b=%v, msg=%s\n", name, b, msg)
	if b != ok {
		return fmt.Errorf("got error reply after writing scp file body: err=%s", err)
	}

	_, err = s.remIn.Write([]byte{ok})
	if err != nil {
		return fmt.Errorf("failed to write scp ok reply: err=%s", err)
	}
	b, msg, err = s.readReply()
	fmt.Printf("replay after writing reply. filename=%s b=%v, msg=%s\n", name, b, msg)
	if b != ok {
		return fmt.Errorf("got error reply after writing scp file body: err=%s", err)
	}

	return nil
}

func (s *source) readReply() (b byte, msg string, err error) {
	return readReply(s.remReader)
}

func readReply(r *bufio.Reader) (b byte, msg string, err error) {
	b, err = r.ReadByte()
	if err != nil {
		return
	}
	if b == ok {
		return
	}
	if b != warning && b != fatalError {
		err = errors.New("unexpected reply type")
		return
	}
	var line []byte
	line, err = r.ReadBytes('\n')
	if err != nil {
		return
	}
	msg = string(line)
	return
}
