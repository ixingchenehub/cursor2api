package ssestream

import "io"

// Event struct represents the event details from the Server-Sent Events(SSE) stream
type Event struct {
	ID   string `json:"id,omitempty"`   // Event ID
	Type string `json:"type,omitempty"` // Event type
	Data []byte `json:"data,omitempty"` // Event data
}

// String returns the data as a string
func (e Event) String() string {
	return string(e.Data)
}

func (e Event) WriteTo(w io.Writer) (int64, error) {
	var written int64
	if e.Type != "" {
		n, _ := io.WriteString(w, "event: ")
		written += int64(n)
		n, _ = io.WriteString(w, e.Type)
		written += int64(n)
		n, _ = w.Write([]byte{'\n'})
		written += int64(n)
	}
	if len(e.Data) > 0 {
		n, _ := io.WriteString(w, "data: ")
		written += int64(n)
		n, _ = w.Write(e.Data)
		written += int64(n)
		n, _ = w.Write([]byte{'\n'})
		written += int64(n)
	}
	n, _ := w.Write([]byte{'\n'})
	written += int64(n)
	return written, nil
}
