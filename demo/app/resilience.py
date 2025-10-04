"""
Production-Grade Resilience Engine
Implements: Retry with Exponential Backoff, Circuit Breaker, Rate Limiting
"""

import asyncio
import time
from dataclasses import dataclass
from enum import Enum
from typing import Any, Callable, Optional

from loguru import logger


class CircuitState(Enum):
    """Circuit breaker states"""
    CLOSED = "closed"      # Normal operation
    OPEN = "open"          # Failed, reject all requests
    HALF_OPEN = "half_open"  # Testing recovery


@dataclass
class RetryConfig:
    """Retry strategy configuration"""
    max_attempts: int = 5
    base_delay: float = 1.0  # Initial delay in seconds
    max_delay: float = 32.0  # Maximum delay cap
    exponential_base: float = 2.0
    jitter: bool = True  # Add randomness to prevent thundering herd
    
    # HTTP status codes that should trigger retry
    retryable_status_codes: tuple = (429, 500, 502, 503, 504)
    
    # Extract Retry-After header
    respect_retry_after: bool = True


@dataclass
class CircuitBreakerConfig:
    """Circuit breaker configuration"""
    failure_threshold: int = 5  # Consecutive failures to open circuit
    success_threshold: int = 2  # Successes in half-open to close circuit
    timeout: float = 60.0  # Seconds before half-open attempt
    

@dataclass
class RateLimiterConfig:
    """Token bucket rate limiter configuration"""
    max_tokens: int = 10  # Maximum concurrent requests
    refill_rate: float = 2.0  # Tokens per second
    

class CircuitBreaker:
    """
    Circuit breaker pattern implementation
    Prevents cascading failures by failing fast when error rate is high
    """
    
    def __init__(self, config: CircuitBreakerConfig):
        self.config = config
        self.state = CircuitState.CLOSED
        self.failure_count = 0
        self.success_count = 0
        self.last_failure_time: Optional[float] = None
        
    def record_success(self):
        """Record successful request"""
        if self.state == CircuitState.HALF_OPEN:
            self.success_count += 1
            logger.info(
                f"🔵 Circuit Breaker: Success in HALF_OPEN state "
                f"({self.success_count}/{self.config.success_threshold})"
            )
            
            if self.success_count >= self.config.success_threshold:
                self._close_circuit()
        elif self.state == CircuitState.CLOSED:
            # Reset failure count on success
            self.failure_count = 0
            
    def record_failure(self):
        """Record failed request"""
        self.last_failure_time = time.time()
        
        if self.state == CircuitState.HALF_OPEN:
            self._open_circuit()
        elif self.state == CircuitState.CLOSED:
            self.failure_count += 1
            logger.warning(
                f"⚠️  Circuit Breaker: Failure count "
                f"({self.failure_count}/{self.config.failure_threshold})"
            )
            
            if self.failure_count >= self.config.failure_threshold:
                self._open_circuit()
                
    def can_request(self) -> bool:
        """Check if request is allowed"""
        if self.state == CircuitState.CLOSED:
            return True
            
        if self.state == CircuitState.OPEN:
            # Check if timeout has elapsed
            if self.last_failure_time and (time.time() - self.last_failure_time) >= self.config.timeout:
                self._half_open_circuit()
                return True
            return False
            
        # HALF_OPEN state
        return True
        
    def _open_circuit(self):
        """Transition to OPEN state"""
        self.state = CircuitState.OPEN
        self.failure_count = 0
        logger.error(
            f"🔴 Circuit Breaker: OPENED - Failing fast for "
            f"{self.config.timeout}s"
        )
        
    def _half_open_circuit(self):
        """Transition to HALF_OPEN state"""
        self.state = CircuitState.HALF_OPEN
        self.success_count = 0
        logger.warning("🟡 Circuit Breaker: HALF_OPEN - Testing recovery")
        
    def _close_circuit(self):
        """Transition to CLOSED state"""
        self.state = CircuitState.CLOSED
        self.failure_count = 0
        self.success_count = 0
        logger.info("🟢 Circuit Breaker: CLOSED - Normal operation resumed")


class TokenBucketRateLimiter:
    """
    Token bucket algorithm for rate limiting
    Prevents overwhelming upstream API with too many concurrent requests
    """
    
    def __init__(self, config: RateLimiterConfig):
        self.config = config
        self.tokens = float(config.max_tokens)
        self.last_refill = time.time()
        self._lock = asyncio.Lock()
        
    async def acquire(self, timeout: float = 30.0) -> bool:
        """
        Acquire a token, waiting if necessary
        Returns True if token acquired, False if timeout
        """
        start_time = time.time()
        
        while True:
            async with self._lock:
                self._refill()
                
                if self.tokens >= 1.0:
                    self.tokens -= 1.0
                    logger.debug(
                        f"🎟️  Token acquired. Remaining: {self.tokens:.1f}"
                    )
                    return True
                    
            # Check timeout
            if (time.time() - start_time) >= timeout:
                logger.warning("⏱️  Rate limiter: Token acquisition timeout")
                return False
                
            # Wait before retry
            await asyncio.sleep(0.1)
            
    def _refill(self):
        """Refill tokens based on time elapsed"""
        now = time.time()
        elapsed = now - self.last_refill
        
        # Add tokens based on refill rate
        refill_amount = elapsed * self.config.refill_rate
        self.tokens = min(
            self.config.max_tokens,
            self.tokens + refill_amount
        )
        self.last_refill = now


class ResilienceEngine:
    """
    Unified resilience engine combining retry, circuit breaker, and rate limiting
    """
    
    def __init__(
        self,
        retry_config: Optional[RetryConfig] = None,
        circuit_config: Optional[CircuitBreakerConfig] = None,
        rate_limiter_config: Optional[RateLimiterConfig] = None
    ):
        self.retry_config = retry_config or RetryConfig()
        self.circuit_breaker = CircuitBreaker(
            circuit_config or CircuitBreakerConfig()
        )
        self.rate_limiter = TokenBucketRateLimiter(
            rate_limiter_config or RateLimiterConfig()
        )
        
        # Metrics
        self.total_requests = 0
        self.successful_requests = 0
        self.failed_requests = 0
        self.retries_count = 0
        
    async def execute(
        self,
        func: Callable,
        *args,
        **kwargs
    ) -> Any:
        """
        Execute function with full resilience protection
        """
        self.total_requests += 1
        
        # Check circuit breaker
        if not self.circuit_breaker.can_request():
            logger.error(
                "🚫 Circuit breaker OPEN - Request rejected to prevent "
                "cascading failure"
            )
            from app.errors import CursorWebError
            raise CursorWebError(
                503,
                "Service temporarily unavailable due to upstream failures. "
                "Circuit breaker is open. Please retry after some time.",
                response_status_code=503
            )
            
        # Acquire rate limiter token
        if not await self.rate_limiter.acquire():
            logger.error("🚫 Rate limiter: Too many concurrent requests")
            from app.errors import CursorWebError
            raise CursorWebError(
                429,
                "Too many requests. Please slow down.",
                response_status_code=429
            )
            
        # Execute with retry logic
        last_exception = None
        
        for attempt in range(1, self.retry_config.max_attempts + 1):
            try:
                logger.info(
                    f"🔄 Attempt {attempt}/{self.retry_config.max_attempts}"
                )
                
                result = await func(*args, **kwargs)
                
                # Success
                self.circuit_breaker.record_success()
                self.successful_requests += 1
                
                if attempt > 1:
                    logger.info(
                        f"✅ Request succeeded after {attempt} attempts"
                    )
                    
                return result
                
            except Exception as e:
                last_exception = e
                
                # Check if retryable
                should_retry = self._should_retry(e, attempt)
                
                if not should_retry:
                    self.circuit_breaker.record_failure()
                    self.failed_requests += 1
                    logger.error(f"❌ Non-retryable error: {e}")
                    raise
                    
                # Calculate delay
                delay = self._calculate_delay(e, attempt)
                
                self.retries_count += 1
                logger.warning(
                    f"⚠️  Attempt {attempt} failed: {e}. "
                    f"Retrying in {delay:.2f}s..."
                )
                
                await asyncio.sleep(delay)
                
        # All retries exhausted
        self.circuit_breaker.record_failure()
        self.failed_requests += 1
        logger.error(
            f"❌ All {self.retry_config.max_attempts} attempts failed"
        )
        raise last_exception
        
    def _should_retry(self, exception: Exception, attempt: int) -> bool:
        """Determine if exception is retryable"""
        # Last attempt never retries
        if attempt >= self.retry_config.max_attempts:
            return False
            
        # Check for CursorWebError with retryable status code
        from app.errors import CursorWebError
        if isinstance(exception, CursorWebError):
            return exception.status_code in self.retry_config.retryable_status_codes
            
        # Network errors are generally retryable
        return False
        
    def _calculate_delay(
        self,
        exception: Exception,
        attempt: int
    ) -> float:
        """Calculate delay before next retry"""
        from app.errors import CursorWebError
        
        # Check for Retry-After header in 429 responses
        if (self.retry_config.respect_retry_after and
            isinstance(exception, CursorWebError) and
            exception.status_code == 429):
            # Try to parse Retry-After from message
            retry_after = self._extract_retry_after(exception)
            if retry_after:
                logger.info(f"📅 Using Retry-After value: {retry_after}s")
                return retry_after
                
        # Exponential backoff
        delay = self.retry_config.base_delay * (
            self.retry_config.exponential_base ** (attempt - 1)
        )
        delay = min(delay, self.retry_config.max_delay)
        
        # Add jitter
        if self.retry_config.jitter:
            import random
            jitter_amount = delay * 0.25  # 25% jitter
            delay += random.uniform(-jitter_amount, jitter_amount)
            
        return max(0.1, delay)  # Minimum 0.1s
        
    def _extract_retry_after(self, exception) -> Optional[float]:
        """Extract Retry-After value from error message"""
        # This is a simplified implementation
        # In production, parse actual HTTP Retry-After header
        try:
            message = str(exception.message) if hasattr(exception, 'message') else str(exception)
            if "retry" in message.lower():
                # Default to 5 seconds if we can't parse
                return 5.0
        except:
            pass
        return None
        
    def get_metrics(self) -> dict:
        """Get resilience metrics"""
        success_rate = (
            (self.successful_requests / self.total_requests * 100)
            if self.total_requests > 0 else 0
        )
        
        return {
            "total_requests": self.total_requests,
            "successful_requests": self.successful_requests,
            "failed_requests": self.failed_requests,
            "success_rate": f"{success_rate:.1f}%",
            "retries_count": self.retries_count,
            "circuit_breaker_state": self.circuit_breaker.state.value,
            "rate_limiter_tokens": f"{self.rate_limiter.tokens:.1f}",
        }


# Global resilience engine instance
_resilience_engine: Optional[ResilienceEngine] = None


def get_resilience_engine() -> ResilienceEngine:
    """Get or create global resilience engine"""
    global _resilience_engine
    
    if _resilience_engine is None:
        # Import here to avoid circular dependency
        try:
            from app.config import (
                RESILIENCE_MAX_RETRIES,
                RESILIENCE_BASE_DELAY,
                RESILIENCE_MAX_DELAY,
                RESILIENCE_CIRCUIT_FAILURE_THRESHOLD,
                RESILIENCE_CIRCUIT_TIMEOUT,
                RESILIENCE_RATE_LIMIT_TOKENS,
                RESILIENCE_RATE_LIMIT_REFILL
            )
            
            retry_config = RetryConfig(
                max_attempts=RESILIENCE_MAX_RETRIES,
                base_delay=RESILIENCE_BASE_DELAY,
                max_delay=RESILIENCE_MAX_DELAY
            )
            
            circuit_config = CircuitBreakerConfig(
                failure_threshold=RESILIENCE_CIRCUIT_FAILURE_THRESHOLD,
                timeout=RESILIENCE_CIRCUIT_TIMEOUT
            )
            
            rate_limiter_config = RateLimiterConfig(
                max_tokens=RESILIENCE_RATE_LIMIT_TOKENS,
                refill_rate=RESILIENCE_RATE_LIMIT_REFILL
            )
            
            _resilience_engine = ResilienceEngine(
                retry_config=retry_config,
                circuit_config=circuit_config,
                rate_limiter_config=rate_limiter_config
            )
            
            logger.info("✅ Resilience Engine initialized successfully")
            
        except ImportError:
            # Fallback to defaults if config not available
            logger.warning("⚠️  Using default resilience configuration")
            _resilience_engine = ResilienceEngine()
            
    return _resilience_engine