# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Cursor2API is a high-performance proxy service that wraps Cursor AI capabilities into OpenAI-compatible API endpoints. The service handles AntiBot bypass, Chrome TLS fingerprinting, and provides both streaming and non-streaming chat completions.

## Architecture

### Core Request Flow

```
Client → CORS Middleware → Auth Middleware → HTTP Handler → Cursor Service → AntiBot Manager → Cursor API
```

### Key Components

**AntiBot Manager** (`models/manager.go`, `models/manager_refresh.go`, `models/manager_api.go`)
- Manages dynamic `x-is-human` authentication parameters required by Cursor API
- Downloads JavaScript from Cursor website and processes it via external Node.js service
- Auto-refresh loop with intelligent idle sleep (wakes up when `GetXIsHuman()` is called after idle timeout)
- Configured refresh interval (default 25s) and idle timeout (default 10min)
- Thread-safe with `sync.RWMutex` for concurrent access

**Cursor Service** (`service/cursor.go`)
- Wraps Cursor API calls with Chrome TLS fingerprinting using `imroc/req` and `refraction-networking/utls`
- Implements both streaming (SSE) and non-streaming chat modes
- Uses `contextReader` wrapper to enable instant cancellation detection for streaming responses
- Converts OpenAI message format to Cursor's expected format via `MessageConverter`

**Message Converter** (`utils/converter.go`)
- Transforms OpenAI-style `messages` array to Cursor's proprietary format
- Injects system prompt as prefix to first user message (configurable via `SYSTEM_PROMPT` env var)
- Handles conversation ID mapping for multi-turn conversations

**Middleware Chain** (`middleware/`)
- `CORS` → `Auth` → `Handler` application order (defined in `main.go:44`)
- API Key authentication validates Bearer tokens against comma-separated `API_KEYS` env var
- Health check endpoint (`/health`) bypasses authentication
- Auth failures return OpenAI-compatible error format with `401` status

**SSE Stream Processing** (`ssestream/`)
- Parses Cursor's SSE event stream format
- Extracts `text-delta` events containing incremental response chunks
- Handles `[DONE]` termination signal

### Configuration System

All configuration loaded from environment variables via `config/config.go`:

```go
// Key environment variables:
PORT                - Server port (default: 3001)
AUTH_ENABLED        - Enable API key auth (default: true)
API_KEYS            - Comma-separated API keys for authentication
JS_URL              - Cursor JavaScript URL for AntiBot parameter extraction
PROCESS_URL         - External Node.js service URL for JS processing (default: http://localhost:3000/api/process)
SYSTEM_PROMPT       - Injected into first user message
REFRESH_INTERVAL    - AntiBot parameter refresh interval in seconds (default: 25)
IDLE_TIMEOUT        - Manager sleep threshold in seconds (default: 600)
LOG_LEVEL           - Logger level: debug/info/warn/error (default: info)
VERBOSE_LOGGING     - Enable detailed request/response logs (default: false)
```

Duration env vars accept both raw seconds (`25`) or Go duration format (`25s`, `10m`).

## Development Commands

### Build & Run
```bash
make build          # Compile to bin/cursor2api
make run            # Run directly without compiling
make dev            # Build + run (development mode)
```

### Code Quality
```bash
make fmt            # Format all Go files
make lint           # Run go vet
make golangci-lint  # Full linting (requires golangci-lint installed)
make test           # Run tests
```

### Docker
```bash
docker-compose up -d                    # Start all services
docker-compose logs -f cursor2api       # View logs
docker-compose down                     # Stop services
```

### Environment Setup
```bash
make env-setup      # Copy .env.example to .env
```

## Critical Implementation Details

### AntiBot Parameter Management
- The service depends on an external Node.js service ([gopkg-dev/x-is-human-api](https://github.com/gopkg-dev/x-is-human-api)) running at `PROCESS_URL`
- This service deobfuscates Cursor's dynamic JavaScript and extracts AntiBot parameters
- `manager.GetXIsHuman()` returns cached parameters and wakes up refresh loop if sleeping
- If refresh fails after `maxRetries` (3), cached value is still returned if available

### Streaming Cancellation Detection
- Streaming uses custom `contextReader` in `service/cursor.go:217` that checks `ctx.Done()` on every `Read()` call
- This enables sub-second response to client disconnects
- All logging for cancellation uses `⚠️` prefix (not `❌`) to indicate expected behavior

### TLS Fingerprinting
- Uses Chrome 131 fingerprint: `req.C().ImpersonateChrome().SetTLSFingerprint(utls.HelloChrome_131)`
- Required headers: `referer`, `x-is-human`, `x-method: POST`, `x-path: /api/chat`
- Fingerprinting setup in `service/cursor.go:28-35`

### API Key Authentication
- Implemented in `middleware/auth.go` (see `docs/api_key_auth/RFC_api_key_auth.md` for full spec)
- Uses constant-time comparison via `crypto/subtle.ConstantTimeCompare` to prevent timing attacks
- Whitelisted paths configured in `main.go:31` (currently only `/health`)
- Failed auth attempts logged with IP and masked key (first 8 chars + `****`)

### Model Mapping
Available models configured in `config/config.go:75-82`:
- `anthropic/claude-4.5-sonnet`
- `anthropic/claude-4-sonnet`
- `anthropic/claude-opus-4.1` (default)
- `openai/gpt-5`
- `google/gemini-2.5-pro`
- `xai/grok-4`

## API Endpoints

### `POST /v1/chat/completions`
OpenAI-compatible chat completion endpoint. Requires Bearer token auth.

**Request:**
```json
{
  "model": "anthropic/claude-opus-4.1",
  "messages": [
    {"role": "user", "content": "Hello"}
  ],
  "stream": false,
  "conversation_id": "optional-session-id"
}
```

**Response (non-streaming):**
Standard OpenAI chat completion response

**Response (streaming):**
SSE format with `data: {...}` events, terminated by `data: [DONE]`

### `GET /v1/models`
Returns list of available models in OpenAI format. Requires auth.

### `GET /health`
Health check endpoint. No authentication required.

## Testing & Debugging

### Checking AntiBot Parameters
- Monitor logs for `♻️ 自动刷新循环已启动` (refresh loop started)
- Look for `✨ 参数刷新成功` (successful refresh)
- If seeing `😴 超过 X 无请求,进入休眠模式`, manager is idle
- `🔔 收到唤醒信号,恢复刷新循环` indicates manager woke up from idle

### Common Issues

**401 Unauthorized:** Check `API_KEYS` env var matches request Bearer token

**AntiBot refresh failures:** Verify `PROCESS_URL` service is running and `JS_URL` is current (JavaScript URL changes periodically - check https://cursor.com/cn/learn)

**Streaming doesn't stop on client disconnect:** Ensure request context is properly passed through handler chain

**Empty responses:** Check `SYSTEM_PROMPT` isn't interfering with expected behavior

## Code Style

- Use structured logging via `logger` package (not raw `fmt.Printf`)
- Log levels: info (`✅`), error (`❌`), warn (`⚠️`), debug (`🔍`)
- All errors should wrap with `fmt.Errorf("context: %w", err)` for stack traces
- HTTP handlers should use helper `writeError()` for consistent error responses
