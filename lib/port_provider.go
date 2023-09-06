/* SPDX-License-Identifier: MIT */
package lib

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

var (
	ErrRange = errors.New("port out of range")
)

func PortChangeNotifier(portFile string, throttleTimeMs uint) (chan uint16, chan struct{}, error) {
	portDir := filepath.Dir(portFile)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, nil, err
	}

	err = watcher.Add(portDir)
	if err != nil {
		return nil, nil, err
	}

	portCh := make(chan uint16)
	quit := make(chan struct{})

	throttleDuration := time.Millisecond * time.Duration(throttleTimeMs)

	go func() {
		defer watcher.Close()
		port, err := GetPortFromFile(portFile)
		if err != nil {
			return
		}
		portCh <- port

		var timer *time.Timer
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Name != portFile {
					continue
				}
				if timer == nil {
					timer = time.AfterFunc(throttleDuration, func() {
						newPort, err := GetPortFromFile(portFile)
						if err != nil {
							fmt.Println("error loading new port file", err)
							return
						}
						if newPort != port {
							port = newPort
							portCh <- port
						}
						timer = nil
					})
				} else {
					timer.Reset(time.Second)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error", err)
			case <-quit:
				return
			}
		}
	}()

	return portCh, quit, nil
}

// WAtch portFile and reparse it when due
func GetPortFromFile(file string) (uint16, error) {
	var port uint16
	fileContent, err := os.ReadFile(file)
	if err != nil {
		return port, err
	}

	port, err = ParsePort(fileContent)
	if err != nil {
		return port, err
	}

	return port, err
}

func ParsePort(fileContent []byte) (uint16, error) {
	trimed := strings.TrimSpace(string(fileContent))
	u64port, err := strconv.ParseUint(trimed, 10, 16)
	port := uint16(u64port)
	if port == 0 || errors.Is(err, strconv.ErrRange) {
		return port, fmt.Errorf("%w - %w", ErrRange, err)
	}
	return port, err
}
