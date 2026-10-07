package chatterm

import (
	"io"
	"net/http"
	"os"
	"time"
)

// Options configures the terminal client for embedding (e.g. games-buddy) or the stock CLI.
type Options struct {
	// AppName selects config under os.UserConfigDir/<AppName>/ (default "simple-chat").
	AppName string
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
	HTTP    *http.Client
	// PlainChat uses POST .../messages/plain instead of .../messages/async.
	PlainChat bool
}

func (o *Options) applyDefaults() {
	if o.AppName == "" {
		o.AppName = "simple-chat"
	}
	if o.Stdin == nil {
		o.Stdin = os.Stdin
	}
	if o.Stdout == nil {
		o.Stdout = os.Stdout
	}
	if o.Stderr == nil {
		o.Stderr = os.Stderr
	}
	if o.HTTP == nil {
		o.HTTP = &http.Client{Timeout: 35 * time.Second}
	}
}

// DefaultOptions returns Options for the reference simple-chat terminal client.
func DefaultOptions() Options {
	return Options{AppName: "simple-chat"}
}
