"""
Production-grade token counter with precise tiktoken-based calculation
"""
import tiktoken
from typing import List, Optional
from loguru import logger
from functools import lru_cache

from app.models import Message


class TokenCounter:
    """Precise token counter using tiktoken library"""
    
    # Model to encoding mapping
    MODEL_ENCODING_MAP = {
        "gpt-4": "cl100k_base",
        "gpt-4-32k": "cl100k_base",
        "gpt-3.5-turbo": "cl100k_base",
        "claude-3": "cl100k_base",
        "claude-2": "cl100k_base",
    }
    
    def __init__(self, model_name: str = "gpt-4"):
        """
        Initialize token counter
        
        Args:
            model_name: Name of the AI model
        """
        self.model_name = model_name
        self.encoding = self._get_encoding(model_name)
    
    @staticmethod
    @lru_cache(maxsize=10)
    def _get_encoding(model_name: str):
        """
        Get tiktoken encoding for model (cached)
        
        Args:
            model_name: Model name
            
        Returns:
            tiktoken.Encoding object
        """
        encoding_name = "cl100k_base"
        for model_prefix, enc_name in TokenCounter.MODEL_ENCODING_MAP.items():
            if model_name.startswith(model_prefix):
                encoding_name = enc_name
                break
        
        try:
            return tiktoken.get_encoding(encoding_name)
        except Exception as e:
            logger.warning(f"Failed to load tiktoken encoding {encoding_name}: {e}. Using fallback.")
            return tiktoken.get_encoding("cl100k_base")
    
    def count_message_tokens(self, message: Message) -> int:
        """
        Count tokens in a single message
        
        Args:
            message: Message object
            
        Returns:
            Token count
        """
        message_overhead = 4
        role_tokens = len(self.encoding.encode(message.role))
        content_tokens = len(self.encoding.encode(message.content or ""))
        return role_tokens + content_tokens + message_overhead
    
    def count_messages_tokens(self, messages: List[Message]) -> int:
        """
        Count total tokens in message list
        
        Args:
            messages: List of messages
            
        Returns:
            Total token count
        """
        if not messages:
            return 0
        total_tokens = sum(self.count_message_tokens(msg) for msg in messages)
        total_tokens += 2
        return total_tokens
    
    def estimate_response_tokens(self, max_tokens: Optional[int] = None) -> int:
        """
        Estimate tokens needed for model response
        
        Args:
            max_tokens: Maximum tokens for response
            
        Returns:
            Estimated response tokens
        """
        if max_tokens:
            return max_tokens
        return 1000


_counter_cache = {}


def get_token_counter(model_name: str = "gpt-4") -> TokenCounter:
    """
    Get or create token counter instance (cached)
    
    Args:
        model_name: Model name
        
    Returns:
        TokenCounter instance
    """
    if model_name not in _counter_cache:
        _counter_cache[model_name] = TokenCounter(model_name)
    return _counter_cache[model_name]