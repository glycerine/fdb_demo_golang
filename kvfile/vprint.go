package kvfile

// Copyright (C) 2022 Jason E. Aten, Ph.D. All rights reserved.

import (
	"fmt"
	"io"
	"os"
	"path"
	"reflect"
	"runtime"
	"runtime/debug"
	"strconv"
	"sync"
	"time"
)

const RFC3339MsecTz0 = "2006-01-02T15:04:05.000Z07:00"

// for tons of debug output
var VerboseVerbose bool = false

// convience functions for . import
var pp = PP
var vv = VV

var MyPID int

func init() {
	MyPID = os.Getpid()
}

func panicOn(err error) {
	if err != nil {
		panic(err)
	}
}

func PP(format string, a ...interface{}) {
	if VerboseVerbose {
		TSPrintf(format, a...)
	}
}

func VV(format string, a ...interface{}) {
	TSPrintf(format, a...)
}

func AlwaysPrintf(format string, a ...interface{}) {
	TSPrintf(format, a...)
}

var tsPrintfMut sync.Mutex

// time-stamped printf
func TSPrintf(format string, a ...interface{}) {
	tsPrintfMut.Lock()
	Printf("\n%s %s ", FileLine(3), ts())
	Printf(format+"\n", a...)
	tsPrintfMut.Unlock()
}

// get timestamp for logging purposes
func ts() string {
	return time.Now().Format(RFC3339MsecTz0)
}

// so we can multi write easily, use our own printf
var OurStdout io.Writer = os.Stdout

// Printf formats according to a format specifier and writes to standard output.
// It returns the number of bytes written and any write error encountered.
func Printf(format string, a ...interface{}) (n int, err error) {
	return fmt.Fprintf(OurStdout, format, a...)
}

func FileLine(depth int) string {
	_, fileName, fileLine, ok := runtime.Caller(depth)
	var s string
	if ok {
		s = fmt.Sprintf("%s:%d", path.Base(fileName), fileLine)
	} else {
		s = ""
	}
	return s
}

func Caller(upStack int) string {
	// elide ourself and runtime.Callers
	target := upStack + 2

	pc := make([]uintptr, target+2)
	n := runtime.Callers(0, pc)

	f := runtime.Frame{Function: "unknown"}
	if n > 0 {
		frames := runtime.CallersFrames(pc[:n])
		for i := 0; i <= target; i++ {
			contender, more := frames.Next()
			if i == target {
				f = contender
			}
			if !more {
				break
			}
		}
	}
	return f.Function
}

var fdlog *os.File

func vvlog(format string, a ...interface{}) {
	var err error
	if fdlog == nil {
		fdlog, err = os.Create(".rbook.vvlog")
		panicOn(err)
	}
	TSFprintf(fdlog, format, a...)
}

// to file handle
func TSFprintf(fd *os.File, format string, a ...interface{}) {
	tsPrintfMut.Lock()
	fmt.Fprintf(fd, "\n%s %s [pid %v] ", FileLine(3), ts(), MyPID)
	fmt.Fprintf(fd, format+"\n", a...)
	tsPrintfMut.Unlock()
}

// return stack dump for calling goroutine.
func stack() string {
	return string(debug.Stack())
}

// return stack dump for all goroutines
func allstacks() string {
	buf := make([]byte, 8192)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			return string(buf[:n])
		}
		buf = make([]byte, 2*len(buf))
	}
}

// IsNil uses reflect to to return true iff the face
// contains a nil pointer, map, array, slice, or channel.
func IsNil(face interface{}) bool {
	if face == nil {
		return true
	}
	switch reflect.TypeOf(face).Kind() {
	case reflect.Ptr, reflect.Array, reflect.Map, reflect.Slice, reflect.Chan:
		return reflect.ValueOf(face).IsNil()
	}
	return false
}

func thisStack() []byte {
	buf := make([]byte, 8192)
	nw := runtime.Stack(buf, false) // false => just us, no other goro.
	buf = buf[:nw]
	return buf
}

// GoroNumber returns the calling goroutine's number.
func GoroNumber() int {
	buf := make([]byte, 48)
	nw := runtime.Stack(buf, false) // false => just us, no other goro.
	buf = buf[:nw]

	// prefix "goroutine " is len 10.
	i := 10
	for buf[i] != ' ' && i < 30 {
		i++
	}
	n, err := strconv.Atoi(string(buf[10:i]))
	panicOn(err)
	return n
}
