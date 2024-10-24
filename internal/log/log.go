package log

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"time"
)

const maxFileSize = 128 * 1024 // 128 KiB

// ANSI escape codes
var Reset, Bold, Red, Green, Yellow, Blue string

var writer io.Writer = &logWriter{inner: os.Stdout}
var debug = os.Getenv("DEBUG") != ""

// writer wraps an io.Writer and implements locking and rotation
type logWriter struct {
	inner io.Writer
	mutex sync.Mutex
}

func init() {
	if runtime.GOOS != "windows" {
		Reset = "\033[0m"
		Bold = "\033[1m"
		Red = "\033[31m"
		Green = "\033[32m"
		Yellow = "\033[33m"
		Blue = "\033[36m"
	}
}

// Write implements io.Writer, locking and rotating as needed
func (w *logWriter) Write(bytes []byte) (int, error) {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	w.tryRotate()
	return w.inner.Write(bytes)
}

func (w *logWriter) tryRotate() {
	f, ok := w.inner.(*os.File)
	if !ok {
		// Not a file, can't rotate
		return
	}
	info, err := f.Stat()
	if err != nil {
		return
	}
	if info.Size() < maxFileSize {
		// Not ripe for rotation
		return
	}
	if f.Truncate(0) == nil {
		f.Seek(0, 0)
	}
}

func timestamp() string {
	currentTime := time.Now()
	format := "15:04:05"
	if debug {
		format = "15:04:05.000"
	}
	return "[" + currentTime.Format(format) + "]"
}

func Debugf(format string, a ...any) {
	if !debug {
		return
	}
	message := fmt.Sprintf(format, a...)
	fmt.Fprintf(writer, "%s DEBUG %s\n", timestamp(), message)
}

func Infof(format string, a ...any) {
	message := fmt.Sprintf(format, a...)
	fmt.Fprintf(writer, "%s %sINFO%s %s\n", timestamp(), Blue, Reset, message)
}

func Warningf(format string, a ...any) {
	message := fmt.Sprintf(format, a...)
	fmt.Fprintf(writer, "%s %sWARNING%s %s\n", timestamp(), Yellow, Reset, message)
}

func Errorf(format string, a ...any) {
	message := fmt.Sprintf(format, a...)
	fmt.Fprintf(writer, "%s %sERROR%s %s\n", timestamp(), Red, Reset, message)
}

func Fatalf(format string, a ...any) {
	message := fmt.Sprintf(format, a...)
	fmt.Fprintf(writer, "%s %sFATAL%s %s\n", timestamp(), Red, Reset, message)
	os.Exit(1)
}

// SetOutput sets the io.Writer to which log messages are written
func SetOutput(w io.Writer) {
	writer = &logWriter{inner: w}
}
