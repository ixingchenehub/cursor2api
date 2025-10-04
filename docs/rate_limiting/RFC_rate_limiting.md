# RFC: Request Rate Limiting Implementation

## Metadata
- **RFC ID**: RFC-001-RATE-LIMITING
- **Status**: Draft
- **Created**: 2025-10-03
- **Author**: AxiomOS
- **Priority**: P1

## Overview

Implement a production-grade request rate limiting mechanism to protect the Cursor2API service from abuse, ensure fair resource allocation, and maintain service availability under high load conditions.

## Goals

1. **Prevent API Abuse**: Implement per-API-key rate limiting to prevent individual clients from overwhelming the service
2. **Fair Resource Allocation**: Ensure all authenticated clients get fair access to service resources
3. **DDoS Protection**: Add IP-based rate limiting as a first line of defense against distributed attacks
4. **Graceful Degradation**: Return clear, OpenAI-compatible error responses when limits are exceeded
5. **Configurable Limits**: Support environment-based configuration for different rate limit tiers
6. **Zero Performance Impact**: Implement using efficient in-memory data structures with minimal overhead

## Non-Goals

1. **Distributed Rate Limiting**: This implementation will be in-memory only (not Redis-based)
2. **Dynamic Limit Adjustment**: Limits are static per configuration, not auto-scaling
3. **Usage Analytics**: Detailed usage tracking is out of scope (covered by P2 Prometheus metrics)
4. **Quota Management**: No monthly/daily quota tracking, only request-per-second limits

## Acceptance Criteria

1. ✅ Rate limiter middleware successfully blocks requests exceeding configured limits
2. ✅ Returns HTTP 429 with OpenAI-compatible error format including `Retry-After` header
3. ✅ Supports both IP-based and API-key-based rate limiting
4. ✅ Configuration via environment variables (e.g., `RATE_LIMIT_PER_SECOND`, `RATE_LIMIT_BURST`)
5. ✅ Zero data races under concurrent load (verified by `go test -race`)
6. ✅ Performance overhead < 1ms per request
7. ✅ Unit test coverage > 95%

## Proposed Solution

### Architecture

We will implement a **Token Bucket** algorithm using Go's `golang.org/x/time/rate` package, which provides:
- Efficient, lock-free implementation
- Built-in burst support
- Proven production reliability

### Component Design

```
┌─────────────────────────────────────────────────────────────┐
│                    HTTP Request Flow                         │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│              CORS Middleware (existing)                      │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│         Rate Limiter Middleware (NEW)                        │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  1. Extract Client ID (IP or API Key)                │   │
│  │  2. Get or Create Token Bucket for Client            │   │
│  │  3. Attempt to Consume 1 Token                       │   │
│  │  4. If Success: Pass to Next Handler                 │   │
│  │  5. If Fail: Return 429 Too Many Requests            │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│           Auth Middleware (existing)                         │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│              API Handlers                                    │
└─────────────────────────────────────────────────────────────┘
```

### Data Structures

```go
// RateLimiter manages rate limiting for multiple clients
type RateLimiter struct {
    limiters sync.Map // map[string]*rate.Limiter
    rate     rate.Limit
    burst    int
    mu       sync.RWMutex
}

// Configuration
type RateLimitConfig struct {
    Enabled        bool
    RequestsPerSec int  // e.g., 10
    BurstSize      int  // e.g., 20
    Strategy       string // "ip" or "api_key"
}
```

### Configuration

New environment variables:
```bash
RATE_LIMIT_ENABLED=true
RATE_LIMIT_REQUESTS_PER_SEC=10
RATE_LIMIT_BURST_SIZE=20
RATE_LIMIT_STRATEGY=api_key  # or "ip"
```

### Error Response Format

```json
{
  "error": {
    "message": "Rate limit exceeded. Please retry after 5 seconds.",
    "type": "rate_limit_error",
    "code": "rate_limit_exceeded"
  }
}
```

HTTP Headers:
```
HTTP/1.1 429 Too Many Requests
Content-Type: application/json
Retry-After: 5
X-RateLimit-Limit: 10
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 1696320000
```

## Alternatives Considered

### Alternative 1: Fixed Window Counter
**Pros**: Simpler implementation
**Cons**: Allows burst at window boundaries (2x limit possible)
**Decision**: Rejected - Token bucket provides better burst control

### Alternative 2: Redis-based Distributed Limiter
**Pros**: Works across multiple instances
**Cons**: Adds external dependency, increased latency
**Decision**: Deferred to future - current single-instance deployment doesn't require it

### Alternative 3: Leaky Bucket
**Pros**: Smoother rate limiting
**Cons**: More complex, doesn't allow controlled bursts
**Decision**: Rejected - Token bucket is more flexible

## Security & Observability Considerations

### Security
1. **IP Spoofing Protection**: Use `X-Forwarded-For` header carefully, validate against trusted proxies
2. **API Key Enumeration**: Rate limit applies after auth, preventing key enumeration attacks
3. **Memory Exhaustion**: Implement periodic cleanup of inactive limiters (TTL: 1 hour)

### Observability
1. **Structured Logging**: Log rate limit violations with client ID and endpoint
2. **Metrics** (P2 task): Expose `rate_limit_exceeded_total` counter
3. **Health Impact**: Monitor if rate limiting affects legitimate traffic

## Implementation Plan

### Phase 1: Core Implementation
1. Create `middleware/ratelimit.go` with Token Bucket implementation
2. Add configuration loading in `config/config.go`
3. Integrate middleware into `main.go` handler chain

### Phase 2: Testing
1. Unit tests for rate limiter logic
2. Concurrent safety tests with `-race` flag
3. Integration tests simulating burst traffic

### Phase 3: Documentation
1. Update README.md with rate limiting configuration
2. Add example `.env` entries

## Risks & Mitigation

| Risk | Impact | Mitigation |
|------|--------|------------|
| Memory leak from unlimited limiter map | High | Implement TTL-based cleanup |
| False positives blocking legitimate users | Medium | Set conservative default limits |
| Performance degradation | Low | Use efficient sync.Map, benchmark |

## Success Metrics

1. **Functional**: 100% of requests exceeding limit receive 429 response
2. **Performance**: < 1ms overhead per request (p99)
3. **Reliability**: Zero panics or data races under load test
4. **Usability**: Clear error messages guide users to retry correctly

## References

- [Token Bucket Algorithm](https://en.wikipedia.org/wiki/Token_bucket)
- [Go rate package](https://pkg.go.dev/golang.org/x/time/rate)
- [OpenAI API Rate Limits](https://platform.openai.com/docs/guides/rate-limits)