// Copyright 2017 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Read, eval, print loop for Notecard interactions

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"os/user"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/blues/note-go/note"
	"github.com/blues/note-go/notecard"
	"github.com/fatih/color"
	"github.com/peterh/liner"
)

type REPL struct {
	context         *notecard.Context
	historyFilePath string
	liner           *liner.State
	format          bool
	watcher         *Watcher
}

func NewREPL(context *notecard.Context) *REPL {
	usr, _ := user.Current()
	repl := &REPL{
		context:         context,
		historyFilePath: path.Join(usr.HomeDir, ".notecard-history"),
		liner:           liner.NewLiner(),
		format:          true,
		watcher:         nil,
	}

	if f, err := os.Open(repl.historyFilePath); err == nil {
		repl.liner.ReadHistory(f)
		f.Close()
	}

	return repl
}

func (repl *REPL) intro() string {
	return `Welcome to the Notecard interactive REPL.
Type 'help' for a list of commands`
}

func (repl *REPL) help() string {
	return `Type any valid JSON command to execute it on the Notecard.

For command reference, visit the documentation at:
https://dev.blues.io/reference/notecard-api/introduction/

Other commands:

watch             Print real time notecard activity
watch [on|off]    If enabled, Notecard activity will be collected in the
                  background.  It can be viewed with the 'watch' command
                  (default: off)
format [on|off]   Auto-format JSON responses (default: on)
history           Show command history
quit              Exit out of the REPL (CTRL-D also exits)`
}

func (repl *REPL) writeHistory() {
	if f, err := os.Create(repl.historyFilePath); err != nil {
		fmt.Println("error writing history file: ", err)
	} else {
		repl.liner.WriteHistory(f)
		f.Close()
	}
}

func (repl *REPL) close() {
	repl.writeHistory()
	repl.liner.Close()
}

// Start the read/eval/print loop which will accept user input
// and execute commands until the user exits.  Returns non-zero
// if it exited because of an error
func (repl *REPL) Start() int {
	defer repl.close()
	fmt.Println(repl.intro())

repl:
	for {
		if input, err := repl.liner.Prompt(">>> "); err == nil {
			if len(input) == 0 {
				continue
			}

			repl.liner.AppendHistory(input)

			normalized := strings.Trim(strings.ToLower(input), " \t")
			normalized = regexp.MustCompile(`\s+`).ReplaceAllString(normalized, " ")

			switch normalized {
			case "quit":
				fallthrough
			case "exit":
				break repl
			case "help":
				fmt.Println(repl.help())
				continue repl
			case "history":
				repl.writeHistory()
				history, err := ioutil.ReadFile(repl.historyFilePath)
				if err != nil {
					fmt.Printf("error: %s\n", err)
				} else {
					fmt.Printf("%s\n", history)
				}
				continue repl
			case "format on":
				fmt.Printf("JSON formatting on\n")
				repl.format = true
				continue repl
			case "format off":
				fmt.Printf("JSON formatting off\n")
				repl.format = false
				continue repl
			case "watch on":
				fmt.Printf("watch mode on\n")
				if repl.watcher != nil {
					continue repl
				}
				repl.watcher = NewWatcher(repl.context)
				continue repl
			case "watch off":
				fmt.Printf("watch mode off\n")
				if repl.watcher == nil {
					continue repl
				}
				repl.watcher.Stop()
				repl.watcher = nil
				continue repl
			case "watch":
				if repl.watcher == nil {
					fmt.Printf("watch mode is off, use 'watch on' to start\n")
					continue repl
				}
				signals := make(chan os.Signal, 1)
				signal.Reset(os.Interrupt)
				signal.Notify(signals, os.Interrupt)
				logsChan := repl.watcher.Channel()
				for {
					select {
					case <-signals:
						repl.watcher.ResetChannel()
						signal.Reset(os.Interrupt)
						continue repl
					case log := <-logsChan:
						fmt.Printf("%s\n", log)
					}
				}
			}

			if isJsonObject(input) {
				// Run the command and print out response
				rspJSON, err := repl.context.TransactionJSON([]byte(input))
				if err != nil {
					fmt.Printf("error: %s\n", err)
					continue repl
				}

				response := string(rspJSON)
				if repl.format {
					var raw map[string]interface{}
					err := json.Unmarshal(rspJSON, &raw)
					if err == nil {
						formatted, err := json.MarshalIndent(raw, "", "    ")
						if err == nil {
							response = string(formatted) + "\n"
						}
					}
				}

				fmt.Printf("%s", string(response))
				continue repl
			}

			fmt.Printf("invalid command: %s\n", input)

		} else if err == liner.ErrPromptAborted || err == io.EOF {
			break
		} else {
			fmt.Printf("error reading line: %s\n", err)
			return 1
		}
	}

	return 0
}

type WatchLogLine struct {
	date      *time.Time
	subsystem string
	message   string
}

func (line WatchLogLine) String() string {
	dateString := "--/--/---- --:--:--"
	if line.date != nil {
		dateString = line.date.Format("01/02/2006 15:04:05")
	}
	return fmt.Sprintf("%s [%-10s] %s", dateString, color.GreenString(line.subsystem), line.message)
}

// A watcher will repeatedly issue the following command in a background goroutine:
//
// {"req": "note.get", "file": "_synclog.qi", "delete": true}
//
// The results are kept in a buffer which can be viewed with the 'watch' REPL command
type Watcher struct {
	logs        []WatchLogLine
	historySize uint16
	bufferSize  uint16
	mutex       sync.Mutex
	done        chan bool
	channel     chan string
}

// Starts a goroutine to monitor for status updates on the notecard
func NewWatcher(context *notecard.Context) *Watcher {
	size := uint16(500)
	buffer := uint16(125)
	watcher := &Watcher{
		nil,
		size,
		buffer,
		sync.Mutex{},
		make(chan bool),
		nil,
	}

	go func(watcher *Watcher) {
		wait := 10 * time.Millisecond
	outer:
		for {
			select {
			case <-watcher.done:
				break outer
			case <-time.After(wait):
			}

			wait = 1000 * time.Millisecond

			req := notecard.Request{Req: "note.get", NotefileID: "_synclog.qi", Delete: true}
			rsp, err := context.TransactionRequest(req)

			if err != nil {
				if !note.ErrorContains(err, note.ErrNoteNoExist) {
					watcher.add(nil, "", fmt.Sprintf("%s", err))
				}
				continue
			}

			if rsp.Body == nil {
				continue
			}

			var bodyJSON []byte
			bodyJSON, err = note.ObjectToJSON(rsp.Body)
			if err != nil {
				watcher.add(nil, "", fmt.Sprintf("[ERROR] %s", err))
				continue
			}

			var body notecard.SyncLogBody
			err = note.JSONUnmarshal(bodyJSON, &body)
			if err != nil {
				watcher.add(nil, "", fmt.Sprintf("[ERROR] %s", err))
				continue
			}

			var date *time.Time
			if body.TimeSecs > 0 {
				parsed := time.Unix(body.TimeSecs, 0)
				date = &parsed
			}
			watcher.add(date, body.Subsystem, body.Text)

			wait = 10 * time.Millisecond
		}
	}(watcher)

	return watcher
}

func (watcher *Watcher) add(date *time.Time, subsystem, message string) {
	watcher.mutex.Lock()
	defer watcher.mutex.Unlock()

	log := WatchLogLine{date, subsystem, message}
	watcher.logs = append(watcher.logs, log)
	if watcher.channel != nil {
		watcher.channel <- log.String()
	}

	if len(watcher.logs) > int(watcher.historySize+watcher.bufferSize) {
		watcher.logs = watcher.logs[watcher.bufferSize:]
	}
}

func (watcher *Watcher) Stop() {
	watcher.done <- true
}

// Returns a channel with log messages suitable for printing to stdout
// Note that subsequent calls to Channel() return the same channel
func (watcher *Watcher) Channel() chan string {
	if watcher.channel == nil {
		watcher.mutex.Lock()
		defer watcher.mutex.Unlock()

		watcher.channel = make(chan string, len(watcher.logs))
		for i := 0; i < len(watcher.logs); i++ {
			watcher.channel <- watcher.logs[i].String()
		}
	}
	return watcher.channel
}

func (watcher *Watcher) ResetChannel() {
	watcher.channel = nil
}

func isJsonObject(s string) bool {
	var raw map[string]interface{}
	return json.Unmarshal([]byte(s), &raw) == nil
}
