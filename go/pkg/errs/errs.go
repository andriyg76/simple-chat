package errs

import (
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
)

const projectPrefix = "github.com/andriyg76/games-buddy"

// TracedError is an error that carries a filtered stack trace captured at construction.
type TracedError struct {
	msg   string
	cause error
	stack string
}

func (e *TracedError) Error() string { return e.msg }
func (e *TracedError) Unwrap() error { return e.cause }

// Is allows errors.Is(err, new(errs.TracedError)) to match any TracedError in the chain.
func (e *TracedError) Is(target error) bool {
	_, ok := target.(*TracedError)
	return ok
}

// LogValue implements slog.LogValuer. When a *TracedError is passed as a slog
// attribute value, it emits a group with "msg" and "stack" fields.
func (e *TracedError) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("msg", e.msg),
		slog.String("stack", e.stack),
	)
}

// Stack returns the pre-formatted filtered stack trace.
func (e *TracedError) Stack() string { return e.stack }

// IsTraced reports whether err's chain contains a *TracedError.
func IsTraced(err error) bool { return errors.Is(err, new(TracedError)) }

// AsTraced returns the first *TracedError in err's chain, or nil.
func AsTraced(err error) *TracedError {
	var te *TracedError
	errors.As(err, &te)
	return te
}

// New creates a new error with a stack trace captured at the call site.
//
//go:noinline
func New(msg string) error {
	return &TracedError{
		msg:   msg,
		stack: captureStack(),
	}
}

// Newf creates a new error using fmt.Errorf formatting (supports %w) with a
// stack trace captured at the call site.
//
//go:noinline
func Newf(format string, args ...any) error {
	cause := fmt.Errorf(format, args...)
	return &TracedError{
		msg:   cause.Error(),
		cause: cause,
		stack: captureStack(),
	}
}

// Wrap wraps err with a stack trace captured at the call site.
// Returns nil if err is nil. Returns err unchanged if it already contains a TracedError.
//
//go:noinline
func Wrap(err error) error {
	if err == nil {
		return nil
	}
	if IsTraced(err) {
		return err
	}
	return &TracedError{
		msg:   err.Error(),
		cause: err,
		stack: captureStack(),
	}
}

func captureStack() string {
	pcs := make([]uintptr, 64)
	n := runtime.Callers(0, pcs)
	pcs = pcs[:n]

	frames := runtime.CallersFrames(pcs)
	var all []runtime.Frame
	for {
		f, more := frames.Next()
		all = append(all, f)
		if !more {
			break
		}
	}

	// Strip only the capture machinery itself, not the whole package.
	internal := map[string]bool{
		"runtime.Callers":                               true,
		projectPrefix + "/internal/errs.captureStack": true,
		projectPrefix + "/internal/errs.Wrap":         true,
		projectPrefix + "/internal/errs.New":          true,
		projectPrefix + "/internal/errs.Newf":         true,
	}
	var filtered []runtime.Frame
	for _, f := range all {
		if internal[f.Function] {
			continue
		}
		filtered = append(filtered, f)
	}

	isProject := func(f runtime.Frame) bool {
		return strings.HasPrefix(f.Function, projectPrefix)
	}

	var buf strings.Builder
	for i, f := range filtered {
		prev := i > 0 && isProject(filtered[i-1])
		curr := isProject(f)
		next := i < len(filtered)-1 && isProject(filtered[i+1])

		if !curr && !prev && !next {
			continue
		}

		if curr {
			fmt.Fprintf(&buf, "\t%s:%d %s\n", f.File, f.Line, f.Function)
		} else {
			fmt.Fprintf(&buf, "\t[%s:%d %s]\n", f.File, f.Line, f.Function)
		}
	}
	return buf.String()
}
