package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"

	"golang.org/x/term"
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
	fmt.Fprintf(e.Stderr, "Warning: "+format+"\n", args...) //nolint:errcheck // best-effort stderr
}

// Error writes an error message to stderr and logs debug info.
func (e *Envelope) Error(err error) {
	if err == nil {
		return
	}

	e.Logger.Write("ERROR: %v", err)

	switch typedErr := err.(type) {
	case *UserError:
		fmt.Fprintln(e.Stderr, typedErr.Error()) //nolint:errcheck // best-effort stderr
	case *AuthError:
		fmt.Fprintln(e.Stderr, typedErr.Error()) //nolint:errcheck // best-effort stderr
	case *InternalError:
		fmt.Fprintf(e.Stderr, "Internal error: %s\n", typedErr.Message) //nolint:errcheck // best-effort stderr
		if e.Logger.Path != "" {
			fmt.Fprintf(e.Stderr, "Debug log: %s\n", e.Logger.Path) //nolint:errcheck // best-effort stderr
		}
	default:
		fmt.Fprintf(e.Stderr, "Error: %v\n", err) //nolint:errcheck // best-effort stderr
		if e.Logger.Path != "" {
			fmt.Fprintf(e.Stderr, "Debug log: %s\n", e.Logger.Path) //nolint:errcheck // best-effort stderr
		}
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
			e.OutputFile.Write(compact)
			e.OutputFile.Write([]byte("\n"))
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
		header.WriteString(fmt.Sprintf("%-*s", widths[i], strings.ToUpper(col)))
	}
	fmt.Fprintln(e.Stdout, header.String())

	// Print rows
	for _, row := range rows {
		var line strings.Builder
		for i, cell := range row {
			if i > 0 {
				line.WriteString("  ")
			}
			if i < len(widths) {
				line.WriteString(fmt.Sprintf("%-*s", widths[i], cell))
			} else {
				line.WriteString(cell)
			}
		}
		fmt.Fprintln(e.Stdout, line.String())
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
func filterFields(data interface{}, fields []string) interface{} {
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
