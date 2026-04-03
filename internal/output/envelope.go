package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"

	"github.com/fatih/color"
	"golang.org/x/term"
)

// Color helpers for TTY output (no-op when piped).
var (
	ColorGreen  = color.New(color.FgGreen)
	ColorRed    = color.New(color.FgRed)
	ColorYellow = color.New(color.FgYellow)
	ColorCyan   = color.New(color.FgCyan)
	ColorDim    = color.New(color.Faint)
)

// Envelope is the output engine that routes data to stdout and messages to stderr.
//
// Behaviour modes:
//   - TTY + no --json flag: table output to stdout, status to stderr
//   - TTY + --json flag:    JSON to stdout, status to stderr
//   - Pipe (non-TTY):       JSON to stdout, status to stderr
//   - DEPLOYHQ_OUTPUT_FILE: JSONL appended to the specified file
type Envelope struct {
	Stdout     io.Writer
	Stderr     io.Writer
	Logger     *Logger
	JSONMode   bool
	JSONFields []string   // field selection for --json
	OutputFile *os.File   // DEPLOYHQ_OUTPUT_FILE JSONL writer
	IsTTY      bool
}

// NewEnvelope creates an Envelope with auto-detected TTY and output file.
func NewEnvelope(logger *Logger) *Envelope {
	e := &Envelope{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Logger: logger,
		IsTTY:  term.IsTerminal(int(os.Stdout.Fd())),
	}

	if path := os.Getenv("DEPLOYHQ_OUTPUT_FILE"); path != "" {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			logger.Write("WARN: could not open DEPLOYHQ_OUTPUT_FILE %q: %v", path, err)
		} else {
			e.OutputFile = f
		}
	}

	return e
}

// Close cleans up resources (output file).
func (e *Envelope) Close() {
	if e.OutputFile != nil {
		_ = e.OutputFile.Close()
	}
}

// Status writes a human-readable message to stderr.
func (e *Envelope) Status(format string, args ...interface{}) {
	fmt.Fprintf(e.Stderr, format+"\n", args...) //nolint:errcheck // best-effort stderr
}

// Warn writes a warning message to stderr.
func (e *Envelope) Warn(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if e.IsTTY {
		ColorYellow.Fprintf(e.Stderr, "Warning: %s\n", msg) //nolint:errcheck
	} else {
		fmt.Fprintf(e.Stderr, "Warning: %s\n", msg) //nolint:errcheck
	}
}

// Error writes an error message to stderr and logs debug info.
func (e *Envelope) Error(err error) {
	if err == nil {
		return
	}

	e.Logger.Write("ERROR: %v", err)

	errorLabel := "Error: "
	if e.IsTTY {
		errorLabel = ColorRed.Sprint("Error: ")
	}

	switch typedErr := err.(type) {
	case *UserError:
		fmt.Fprint(e.Stderr, errorLabel+typedErr.Message+"\n") //nolint:errcheck
		if typedErr.Hint != "" {
			if e.IsTTY {
				ColorDim.Fprintf(e.Stderr, "\nHint: %s\n", typedErr.Hint) //nolint:errcheck
			} else {
				fmt.Fprintf(e.Stderr, "\nHint: %s\n", typedErr.Hint) //nolint:errcheck
			}
		}
	case *AuthError:
		fmt.Fprint(e.Stderr, errorLabel+typedErr.Message+"\n") //nolint:errcheck
		if typedErr.Hint != "" {
			if e.IsTTY {
				ColorDim.Fprintf(e.Stderr, "\nHint: %s\n", typedErr.Hint) //nolint:errcheck
			} else {
				fmt.Fprintf(e.Stderr, "\nHint: %s\n", typedErr.Hint) //nolint:errcheck
			}
		}
	case *InternalError:
		fmt.Fprintf(e.Stderr, "%sInternal error: %s\n", errorLabel, typedErr.Message) //nolint:errcheck
		if e.Logger.Path != "" {
			fmt.Fprintf(e.Stderr, "Debug log: %s\n", e.Logger.Path) //nolint:errcheck
		}
	default:
		fmt.Fprintf(e.Stderr, "%s%v\n", errorLabel, err) //nolint:errcheck
		if e.Logger.Path != "" {
			fmt.Fprintf(e.Stderr, "Debug log: %s\n", e.Logger.Path) //nolint:errcheck
		}
	}
}

// ColorStatus returns a colorized status string for TTY output.
func ColorStatus(status string) string {
	switch status {
	case "completed", "online", "enabled", "yes":
		return ColorGreen.Sprint(status)
	case "failed", "error", "offline", "disabled", "no":
		return ColorRed.Sprint(status)
	case "running", "pending", "queued", "building", "transferring":
		return ColorYellow.Sprint(status)
	default:
		return status
	}
}

// WriteJSON writes data as JSON to stdout and optionally to the JSONL output file.
// If JSONFields is set, only those fields are included.
func (e *Envelope) WriteJSON(data interface{}) error {
	output := data
	if len(e.JSONFields) > 0 {
		output = filterFields(data, e.JSONFields)
	}

	enc := json.NewEncoder(e.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(output); err != nil {
		return fmt.Errorf("encode json: %w", err)
	}

	// Write to JSONL output file (compact, one line per record)
	if e.OutputFile != nil {
		compact, err := json.Marshal(output)
		if err == nil {
			_, _ = e.OutputFile.Write(compact)
			_, _ = e.OutputFile.Write([]byte("\n"))
		}
	}

	return nil
}

// WriteTable writes data in a human-readable table format.
// columns defines the header names, and rows provides the data.
func (e *Envelope) WriteTable(columns []string, rows [][]string) {
	if len(rows) == 0 {
		return
	}

	// Calculate column widths
	widths := make([]int, len(columns))
	for i, col := range columns {
		widths[i] = len(col)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Print header
	var header strings.Builder
	for i, col := range columns {
		if i > 0 {
			header.WriteString("  ")
		}
		fmt.Fprintf(&header, "%-*s", widths[i], strings.ToUpper(col)) //nolint:errcheck
	}
	fmt.Fprintln(e.Stdout, header.String()) //nolint:errcheck

	// Print rows
	for _, row := range rows {
		var line strings.Builder
		for i, cell := range row {
			if i > 0 {
				line.WriteString("  ")
			}
			if i < len(widths) {
				fmt.Fprintf(&line, "%-*s", widths[i], cell) //nolint:errcheck
			} else {
				line.WriteString(cell)
			}
		}
		fmt.Fprintln(e.Stdout, line.String()) //nolint:errcheck
	}
}

// WriteData writes data as either JSON or table depending on the mode.
// If in JSON mode or non-TTY, outputs JSON. Otherwise, uses the table formatter.
func (e *Envelope) WriteData(data interface{}, columns []string, toRow func(interface{}) []string) error {
	if e.JSONMode || !e.IsTTY {
		return e.WriteJSON(data)
	}

	// Convert data to rows for table output
	val := reflect.ValueOf(data)
	if val.Kind() == reflect.Slice {
		rows := make([][]string, val.Len())
		for i := 0; i < val.Len(); i++ {
			rows[i] = toRow(val.Index(i).Interface())
		}
		e.WriteTable(columns, rows)
	} else {
		// Single item
		e.WriteTable(columns, [][]string{toRow(data)})
	}
	return nil
}

// filterFields extracts only the specified fields from a JSON-serializable value.
// If the value is a Response envelope, it unwraps and filters the Data field.
func filterFields(data interface{}, fields []string) interface{} {
	// Unwrap Response envelope — filter the inner Data, not the wrapper
	if resp, ok := data.(*Response); ok {
		return filterFields(resp.Data, fields)
	}

	// Marshal to map, then pick fields
	b, err := json.Marshal(data)
	if err != nil {
		return data
	}

	// Try as array
	var arr []map[string]interface{}
	if json.Unmarshal(b, &arr) == nil && arr != nil {
		result := make([]map[string]interface{}, len(arr))
		for i, item := range arr {
			result[i] = pickFields(item, fields)
		}
		return result
	}

	// Try as object
	var obj map[string]interface{}
	if json.Unmarshal(b, &obj) == nil && obj != nil {
		return pickFields(obj, fields)
	}

	return data
}

func pickFields(m map[string]interface{}, fields []string) map[string]interface{} {
	result := make(map[string]interface{}, len(fields))
	for _, f := range fields {
		if v, ok := m[f]; ok {
			result[f] = v
		}
	}
	return result
}
