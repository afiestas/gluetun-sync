/* SPDX-License-Identifier: MIT */
package lib

import (
	"fmt"
	"net/url"

	"github.com/fatih/color"
)

var (
	bold color.Color = *color.New(color.Bold)
)

func Info(info string) {
	bold.Println(info)
}

func printRequesterError(err error) {
	if err == nil {
		return
	}

	switch err := err.(type) {
	case *url.Error:
		PrintStepError(err.Err)
	default:
		PrintStepError(err)
	}
}

func PrintStepError(err error) {
	color.Red("     %s\n", err)
}

// Channel will be closed by requester :/
func PrintRequester() (chan StatusUpdate, chan struct{}) {
	updateCh := make(chan StatusUpdate)
	quitCh := make(chan struct{})
	go func() {
		for update := range updateCh {
			if update.Step == 1 {
				fmt.Printf("🔁 Service %s\n", update.Service)
			}
			if update.Status == UnInitialized {
				continue
			}
			status := "✅"
			if update.Status == Error {
				status = "❌"
			}
			fmt.Printf("  └─ %s %s %s\n", update.Method, update.Path, status)
			if update.Error != nil {
				printRequesterError(update.Error)
			}
		}
		close(quitCh)
	}()

	return updateCh, quitCh
}

func PrintError(err error) {
	color.Red("%s", err)
}
