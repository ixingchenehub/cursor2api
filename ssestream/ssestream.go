package ssestream

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"time"
)

var (
	defaultSseMaxBufSize = 1 << 15 // 32KB
	defaultEventName     = "claude"

	headerID    = []byte("id:")
	headerData  = []byte("data:")
	headerEvent = []byte("event:")
	headerRetry = []byte("retry:")

	// ErrEmptyMessage 表示SSE流中的空消息，这是正常的分隔符
	ErrEmptyMessage = errors.New("sse: event claude was empty")
)

type (
	// EventMessageFunc is a callback function type used to receive event details
	// from the Server-Sent Events(SSE) stream
	EventMessageFunc func(any)

	// EventErrorFunc is a callback function type used to receive notification
	// when an error occurs with EventSource processing
	EventErrorFunc func(error)

	// EventProcessor provides callback-based event processing similar to Resty v3 EventSource
	EventProcessor interface {
		// SetMaxBufSize sets the buffer size for SSE processing
		SetMaxBufSize(int) EventProcessor
		// SetWriter sets the writer for automatic event forwarding
		SetWriter(io.Writer) EventProcessor
		// OnMessage registers a callback for default "claude" events
		OnMessage(EventMessageFunc, any) EventProcessor
		// AddEventListener registers a callback for specific event types
		AddEventListener(string, EventMessageFunc, any) EventProcessor
		// OnError registers a callback for error handling
		OnError(EventErrorFunc) EventProcessor
		// StartStreaming starts processing events from the HTTP response
		StartStreaming(r *http.Response) error
		// GetStats returns the forwarding statistics
		GetStats() (totalWritten int64, chunkCount int)
		// Close closes the event processor
		Close()
	}

	callback struct {
		Func   EventMessageFunc
		Result any
	}
)

// eventProcessor implements EventProcessor interface, similar to Resty v3 EventSource
type eventProcessor struct {
	lock            *sync.RWMutex
	maxBufSize      int
	writer          io.Writer    // 用于自动转发事件
	flusher         http.Flusher // 用于流式刷新
	totalWritten    int64        // 转发的字节数统计
	chunkCount      int          // 转发的块数统计
	onEvent         map[string]*callback
	onError         EventErrorFunc
	closed          bool
	lastEventID     string
	serverSentRetry time.Duration
}

// NewEventProcessor creates a new event processor directly from HTTP response,
func NewEventProcessor() EventProcessor {
	return &eventProcessor{
		maxBufSize: defaultSseMaxBufSize,
		onEvent:    make(map[string]*callback),
		lock:       &sync.RWMutex{},
	}
}

// SetMaxBufSize method sets the given buffer size into the SSE client
//
// Default is 32kb
//
//	es.SetMaxBufSize(64 * 1024) // 64kb
func (ep *eventProcessor) SetMaxBufSize(bufSize int) EventProcessor {
	ep.lock.Lock()
	defer ep.lock.Unlock()
	ep.maxBufSize = bufSize
	return ep
}

// SetWriter method sets the writer for automatic event forwarding
// If the writer implements http.Flusher, it will be used for streaming
//
//	es.SetWriter(responseWriter)
func (ep *eventProcessor) SetWriter(w io.Writer) EventProcessor {
	ep.lock.Lock()
	defer ep.lock.Unlock()
	ep.writer = w

	// 检查是否支持Flusher接口
	if flusher, ok := w.(http.Flusher); ok {
		ep.flusher = flusher
	}

	return ep
}

// OnMessage registers a callback for default "claude" events
func (ep *eventProcessor) OnMessage(fn EventMessageFunc, result any) EventProcessor {
	return ep.AddEventListener(defaultEventName, fn, result)
}

// OnError registered callback gets triggered when the error occurred
// in the process
//
//	es.OnError(func(err error) {
//		fmt.Println("Error occurred:", err)
//	})
func (ep *eventProcessor) OnError(fn EventErrorFunc) EventProcessor {
	ep.lock.Lock()
	defer ep.lock.Unlock()
	if ep.onError != nil {
		slog.Warn("Overwriting an existing OnError callback",
			"from", functionName(ep.OnError), "to", functionName(fn))
	}
	ep.onError = fn
	return ep
}

// AddEventListener method registers a callback to consume a specific event type
// messages from the server. The second result argument is optional; it can be used
// to register the data type for JSON data.
//
//	es.AddEventListener(
//		"friend_logged_in",
//		func(e any) {
//			e = e.(*Event)
//			fmt.Println(e)
//		},
//		nil,
//	)
//
//	// Receiving JSON data from the server, you can set result type
//	// to do auto-unmarshal
//	es.AddEventListener(
//		"friend_logged_in",
//		func(e any) {
//			e = e.(*UserLoggedIn)
//			fmt.Println(e)
//		},
//		UserLoggedIn{},
//	)
func (ep *eventProcessor) AddEventListener(eventName string, fn EventMessageFunc, result any) EventProcessor {
	ep.lock.Lock()
	defer ep.lock.Unlock()
	if e, found := ep.onEvent[eventName]; found {
		slog.Warn("Overwriting an existing OnEvent callback",
			"from", functionName(e), "to", functionName(fn))
	}
	cb := &callback{Func: fn, Result: nil}
	if result != nil {
		cb.Result = getPointer(result)
	}
	ep.onEvent[eventName] = cb
	return ep
}

// StartStreaming starts processing events from the HTTP response, similar to Resty v3 EventSource
func (ep *eventProcessor) StartStreaming(resp *http.Response) error {
	if resp == nil || resp.Body == nil {
		return nil
	}

	defer closeq(resp.Body) //nolint:errcheck

	if len(ep.onEvent) == 0 {
		return fmt.Errorf("resty:sse: At least one OnMessage/AddEventListener func is required")
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, slices.Min([]int{4096, ep.maxBufSize})), ep.maxBufSize)
	scanner.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}
		if i := bytes.Index(data, []byte{'\n', '\n'}); i >= 0 {
			// We have a full double newline-terminated line.
			return i + 1, data[0:i], nil
		}
		// If we're at EOF, we have a final, non-terminated line. Return it.
		if atEOF {
			return len(data), data, nil
		}
		// Request more data.
		return 0, nil, nil
	})

	for {
		if ep.isClosed() {
			slog.Debug("SSE stream processor closed")
			return nil
		}

		if err := ep.processEvent(scanner); err != nil {
			if errors.Is(err, io.EOF) {
				slog.Debug("SSE stream ended normally")
				return nil // 正常结束
			}
			slog.Debug("SSE stream error", "error", err)
			return err
		}
	}
}

func (ep *eventProcessor) processEvent(scanner *bufio.Scanner) error {
	e, err := readEvent(scanner)
	if err != nil {
		if errors.Is(err, io.EOF) {
			slog.Debug("SSE stream reached EOF, closing normally")
			return io.EOF // 返回 EOF 而不是 nil，让调用者知道流结束了
		}
		ep.triggerOnError(err)
		return err
	}

	ed, err := parseEvent(e)
	if err != nil {
		if errors.Is(err, ErrEmptyMessage) {
			// 空消息是正常的，直接忽略，不记录错误
			return nil
		}
		ep.triggerOnError(err)
		return nil // parsing errors, will not return error.
	}
	defer putRawEvent(ed)

	if len(ed.ID) > 0 {
		ep.lock.Lock()
		ep.lastEventID = string(ed.ID)
		ep.lock.Unlock()
	}

	if len(ed.Retry) > 0 {
		if retry, err := strconv.Atoi(string(ed.Retry)); err == nil {
			ep.lock.Lock()
			ep.serverSentRetry = time.Millisecond * time.Duration(retry)
			ep.lock.Unlock()
		} else {
			ep.triggerOnError(err)
		}
	}

	if len(ed.Data) > 0 {
		ep.handleCallback(&Event{
			ID:   string(ed.ID),
			Type: string(ed.Event),
			Data: ed.Data,
		})
	}

	return nil
}

func (ep *eventProcessor) handleCallback(e *Event) {
	ep.lock.Lock()
	defer ep.lock.Unlock()

	// 自动转发事件到writer（如果设置了）
	if ep.writer != nil {
		if n, err := e.WriteTo(ep.writer); err != nil {
			ep.triggerOnError(fmt.Errorf("failed to write event: %w", err))
		} else {
			ep.totalWritten += n
			ep.chunkCount++

			// 如果支持flush，立即刷新
			if ep.flusher != nil {
				ep.flusher.Flush()
			}
		}
	}

	// 处理回调函数
	eventName := e.Type
	if len(eventName) == 0 {
		eventName = defaultEventName
	}
	if cb, found := ep.onEvent[eventName]; found {
		if cb.Result == nil {
			cb.Func(e)
			return
		}
		r := newInterface(cb.Result)
		if err := decodeJSON(bytes.NewReader(e.Data), r); err != nil {
			ep.triggerOnError(err)
			return
		}
		cb.Func(r)
	}
}

// GetStats returns the forwarding statistics
func (ep *eventProcessor) GetStats() (totalWritten int64, chunkCount int) {
	ep.lock.RLock()
	defer ep.lock.RUnlock()
	return ep.totalWritten, ep.chunkCount
}

// Close closes the event processor
func (ep *eventProcessor) Close() {
	ep.lock.Lock()
	defer ep.lock.Unlock()
	ep.closed = true
}

func (ep *eventProcessor) isClosed() bool {
	ep.lock.RLock()
	defer ep.lock.RUnlock()
	return ep.closed
}

func (ep *eventProcessor) triggerOnError(err error) {
	ep.lock.RLock()
	defer ep.lock.RUnlock()
	if ep.onError != nil {
		ep.onError(err)
	}
}

var readEvent = readEventFunc

func readEventFunc(scanner *bufio.Scanner) ([]byte, error) {
	if scanner.Scan() {
		event := scanner.Bytes()
		return event, nil
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return nil, io.EOF
}

type rawEvent struct {
	ID    []byte
	Data  []byte
	Event []byte
	Retry []byte
}

var parseEvent = parseEventFunc

// event value parsing logic obtained and modified for Resty processing flow.
// https://github.com/r3labs/sse/blob/c6d5381ee3ca63828b321c16baa008fd6c0b4564/client.go#L322
func parseEventFunc(msg []byte) (*rawEvent, error) {
	// 空消息是正常的，按照SSE规范应该忽略而不是报错
	if len(msg) < 1 {
		return nil, ErrEmptyMessage
	}

	e := newRawEvent()

	// Split the line by "\n"
	for _, line := range bytes.FieldsFunc(msg, func(r rune) bool { return r == '\n' }) {
		switch {
		case bytes.HasPrefix(line, headerID):
			e.ID = append([]byte(nil), trimHeader(len(headerID), line)...)
		case bytes.HasPrefix(line, headerData):
			// The spec allows for multiple data fields per event, concatenated them with "\n"
			e.Data = append(e.Data[:], append(trimHeader(len(headerData), line), byte('\n'))...)
		// The spec says that a line that simply contains the string "data" should be treated as a data field with an empty body.
		case bytes.Equal(line, bytes.TrimSuffix(headerData, []byte(":"))):
			e.Data = append(e.Data, byte('\n'))
		case bytes.HasPrefix(line, headerEvent):
			e.Event = append([]byte(nil), trimHeader(len(headerEvent), line)...)
		case bytes.HasPrefix(line, headerRetry):
			e.Retry = append([]byte(nil), trimHeader(len(headerRetry), line)...)
		default:
			// Ignore anything that doesn't match the header
		}
	}

	// Trim the last "\n" per the spec
	e.Data = bytes.TrimSuffix(e.Data, []byte("\n"))

	return e, nil
}

func trimHeader(size int, data []byte) []byte {
	if data == nil || len(data) < size {
		return data
	}
	data = data[size:]
	data = bytes.TrimSpace(data)
	data = bytes.TrimSuffix(data, []byte("\n"))
	return data
}

var rawEventPool = &sync.Pool{New: func() any { return new(rawEvent) }}

func newRawEvent() *rawEvent {
	e := rawEventPool.Get().(*rawEvent)
	e.ID = e.ID[:0]
	e.Data = e.Data[:0]
	e.Event = e.Event[:0]
	e.Retry = e.Retry[:0]
	return e
}

func putRawEvent(e *rawEvent) {
	rawEventPool.Put(e)
}
