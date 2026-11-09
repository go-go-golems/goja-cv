package graph

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
)

// Error and warning codes (guide Part 4, Appendix B, plus the M1 additions
// E_NO_FIELD, E_IMMUTABLE, E_EVAL and E_INTERNAL and the M3 addition
// E_PLACEHOLDER).
const (
	EUnknownMethod    = "E_UNKNOWN_METHOD"
	ENamespace        = "E_NAMESPACE"
	EWrongKind        = "E_WRONG_KIND"
	EArgType          = "E_ARG_TYPE"
	EArgRange         = "E_ARG_RANGE"
	EUnknownOption    = "E_UNKNOWN_OPTION"
	ELazyCompare      = "E_LAZY_COMPARE"
	EParamRedeclare   = "E_PARAM_REDECLARE"
	ETimeout          = "E_TIMEOUT"
	ECancelled        = "E_CANCELLED"
	ENoField          = "E_NO_FIELD"    // a derived field does not exist in the evaluated value
	EImmutable        = "E_IMMUTABLE"   // assignment to a node property
	EEval             = "E_EVAL"        // a kernel or IO failure during evaluation
	EInternal         = "E_INTERNAL"    // a bug: an op panicked
	EPlaceholder      = "E_PLACEHOLDER" // a builder lambda's placeholder node was evaluated outside its builder
	EFrame            = "E_FRAME"       // images combined by one op cover different frames
	EFormat           = "E_FORMAT"      // a file feature goja-cv does not read
	ETooMany          = "E_TOO_MANY"    // an awaited result would be too large for JavaScript
	WUnusedNode       = "UNUSED_NODE"
	WRange            = "RANGE"
	WCropFirst        = "CROP_FIRST"       // a filter with a halo after a crop sees only the window
	WDecorationFirst  = "DECORATION_FIRST" // a pixel op after a decoration sees the drawn marks
	WCut              = "CUT"
	WLooks            = "LOOKS" // looks beyond the session's per-eval budget were not delivered
	WData             = "DATA"  // data items beyond the session's per-eval budget were not delivered
	pathFailureMarker = "✗ "
)

// Error is the structured error every cv failure is reported as. It crosses
// into JavaScript as an Error object carrying these fields.
type Error struct {
	Code    string   `json:"code"`
	Message string   `json:"message"`
	Path    []string `json:"path,omitempty"` // node path: open(scan.tif) → exposure → ✗ gaussian
	Hint    string   `json:"hint,omitempty"`
	Fix     string   `json:"fix,omitempty"`
	Valid   []string `json:"valid,omitempty"` // valid alternatives (option keys, methods, enum values)
	Line    int      `json:"line,omitempty"`  // filled by the eval tool from the JS stack
	cause   error
	field   string // the unknown field of an UnknownFieldError (for the producer hint)
}

var _ error = &Error{}

func (e *Error) Error() string {
	var sb strings.Builder
	sb.WriteString(e.Code)
	sb.WriteString(": ")
	sb.WriteString(e.Message)
	if len(e.Path) > 0 {
		sb.WriteString(" (at ")
		sb.WriteString(strings.Join(e.Path, " → "))
		sb.WriteString(")")
	}
	return sb.String()
}

// Unwrap returns the underlying Go error, if any.
func (e *Error) Unwrap() error { return e.cause }

// Errorf builds an Error with a formatted message.
func Errorf(code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// WithHint sets the hint and returns e.
func (e *Error) WithHint(h string) *Error { e.Hint = h; return e }

// WithFix sets the literal fix and returns e.
func (e *Error) WithFix(f string) *Error { e.Fix = f; return e }

// WithValid sets the list of valid alternatives and returns e.
func (e *Error) WithValid(v []string) *Error { e.Valid = v; return e }

// AsError extracts a *Error from err's chain.
func AsError(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// A prefixes word with its indefinite article: A("Image") = "an Image",
// A("Mask") = "a Mask". It looks at the first letter only, which is right for
// the kind names, types and op names it is used with.
func A(word string) string {
	if word == "" {
		return word
	}
	switch word[0] {
	case 'a', 'e', 'i', 'o', 'u', 'A', 'E', 'I', 'O', 'U':
		return "an " + word
	}
	return "a " + word
}

// Warning travels with results instead of failing them.
type Warning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (w Warning) String() string { return w.Code + ": " + w.Message }
