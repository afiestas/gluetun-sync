/* SPDX-License-Identifier: MIT */
package lib

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"testing"
)

func TestGetPortFromFileFileNotFound(t *testing.T) {
	_, err := GetPortFromFile("/some/non/existen/path")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("The returned error should be file not found: %s", err)
	}
}

func TestParsePortNotInt(t *testing.T) {
	_, err := ParsePort([]byte("someword"))
	if !errors.Is(err, strconv.ErrSyntax) {
		t.Fatalf("The returned error should be ErrSyntax: %s", err)
	}
}

func TestParsePortTrim(t *testing.T) {
	port, err := ParsePort([]byte("\t\n 1111 \n"))
	if err != nil {
		t.Fatalf("The returned error should be ErrSyntax: %s", err)
	}
	if port != 1111 {
		t.Fatalf("Port should be 1111 but %d returned ", port)
	}
}

func TestParsePortIntOutOfRange(t *testing.T) {
	const minTcpPort = 1
	const maxTcpPort = 65535
	_, err := ParsePort([]byte(fmt.Sprintf("%d", maxTcpPort+1)))
	if !errors.Is(err, ErrRange) {
		t.Fatalf("The returned error should be ErrRange: %s", err)
	}

	_, err = ParsePort([]byte(fmt.Sprintf("%d", minTcpPort-1)))
	if !errors.Is(err, ErrRange) {
		t.Fatalf("The returned error should be ErrRange: %s", err)
	}

	_, err = ParsePort([]byte("-1"))
	if !errors.Is(err, ErrRange) {
		t.Fatalf("The returned error should be ErrRange: %s", err)
	}
}

// This test is racy if system is unablet to  modify files within 100ms
func TestPortChangeNotifier(t *testing.T) {
	file, err := os.CreateTemp("", "portfile-")
	if err != nil {
		t.Fatalf("couldn't create temp file for portifle watching test %v", err)
	}

	fileInfo, err := file.Stat()
	if err != nil {
		t.Fatalf("couldn't stat temp file for portifle watching test %v", err)
	}
	mode := fileInfo.Mode()
	fileName := file.Name()
	file.Close()

	os.WriteFile(fileName, []byte("1337"), mode)

	portCh, quit, err := PortChangeNotifier(file.Name(), 100)
	if err != nil {
		t.Fatalf("portChangeNotifier failed %v", err)
	}

	t.Run("Initial value", func(t *testing.T) {
		port := <-portCh
		if port != 1337 {
			t.Fatalf("expected first port to be 1337 but instead it was %d", port)
		}
	})

	t.Run("Batch of two writes", func(t *testing.T) {
		os.WriteFile(fileName, []byte("1338"), mode)
		os.WriteFile(fileName, []byte("1339"), mode)
		port := <-portCh
		if port != 1339 {
			t.Fatalf("expected second port to be 1339 but instead it was %d", port)
		}
	})

	t.Run("Recreate file with 3 writes", func(t *testing.T) {
		os.Remove(file.Name())
		file, err := os.OpenFile(fileName, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, mode)
		if err != nil {
			t.Fatalf("couldn't recreate file %v", err)
		}
		defer file.Close()
		file.Write([]byte("1"))
		file.Seek(0, 0)
		file.Write([]byte("2"))
		file.Seek(0, 0)
		file.Write([]byte("3"))
		file.Seek(0, 0)
		file.Close()
		port := <-portCh
		if port != 3 {
			t.Fatalf("expected third port to be 3 but instead it was %d", port)
		}
	})

	t.Cleanup(func() {
		close(quit)
		os.Remove(file.Name())
	})
}
