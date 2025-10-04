"""
Intelligent message compressor with multi-level compression strategies
"""
from typing import List, Tuple, Dict
from loguru import logger

from app.models import Message
from app.token_counter import get_token_counter
from app.config import MAX_TOKENS


class MessageCompressor:
    """Multi-level intelligent message compression"""
    
    def __init__(self, model_name: str = "gpt-4"):
        """
        Initialize compressor
        
        Args:
            model_name: Model name for token counting
        """
        self.model_name = model_name
        self.counter = get_token_counter(model_name)
    
    def compress(
        self,
        messages: List[Message],
        max_tokens: int = MAX_TOKENS,
        reserve_ratio: float = 0.1
    ) -> Tuple[List[Message], Dict]:
        """
        Auto-compress messages to fit token limit
        
        Args:
            messages: Original message list
            max_tokens: Maximum allowed tokens
            reserve_ratio: Reserve ratio for response (0.1 = 10%)
            
        Returns:
            Tuple of (compressed_messages, compression_stats)
        """
        if not messages:
            return messages, {"level": 0, "original_tokens": 0, "final_tokens": 0}
        
        original_tokens = self.counter.count_messages_tokens(messages)
        target_tokens = int(max_tokens * (1 - reserve_ratio))
        
        stats = {
            "level": 0,
            "original_tokens": original_tokens,
            "final_tokens": original_tokens,
            "removed_messages": 0,
            "compression_ratio": 0.0
        }
        
        if original_tokens <= target_tokens:
            logger.info(f"✅ No compression needed: {original_tokens}/{target_tokens} tokens")
            return messages, stats
        
        logger.warning(f"⚠️  Token limit exceeded: {original_tokens}/{target_tokens}, starting compression...")
        
        result = self._level1_remove_old(messages, target_tokens)
        if result:
            compressed, level = result
            stats.update({
                "level": level,
                "final_tokens": self.counter.count_messages_tokens(compressed),
                "removed_messages": len(messages) - len(compressed)
            })
            stats["compression_ratio"] = (stats["original_tokens"] - stats["final_tokens"]) / stats["original_tokens"]
            logger.info(f"✅ Compression completed: Level {level}, {stats['final_tokens']}/{target_tokens} tokens")
            return compressed, stats
        
        result = self._level2_keep_recent(messages, target_tokens)
        if result:
            compressed, level = result
            stats.update({
                "level": level,
                "final_tokens": self.counter.count_messages_tokens(compressed),
                "removed_messages": len(messages) - len(compressed)
            })
            stats["compression_ratio"] = (stats["original_tokens"] - stats["final_tokens"]) / stats["original_tokens"]
            logger.info(f"✅ Compression completed: Level {level}, {stats['final_tokens']}/{target_tokens} tokens")
            return compressed, stats
        
        logger.error("🚨 Emergency compression: keeping only system + last message")
        system_msgs = [m for m in messages if m.role == "system"]
        last_msg = [messages[-1]] if messages else []
        compressed = system_msgs + last_msg
        
        stats.update({
            "level": 3,
            "final_tokens": self.counter.count_messages_tokens(compressed),
            "removed_messages": len(messages) - len(compressed),
            "compression_ratio": (stats["original_tokens"] - stats["final_tokens"]) / stats["original_tokens"]
        })
        
        return compressed, stats
    
    def _level1_remove_old(self, messages: List[Message], target: int) -> Tuple[List[Message], int]:
        """Level 1: Remove oldest non-system messages"""
        system = [m for m in messages if m.role == "system"]
        non_system = [m for m in messages if m.role != "system"]
        
        compressed = system + non_system
        while len(non_system) > 1:
            tokens = self.counter.count_messages_tokens(compressed)
            if tokens <= target:
                return compressed, 1
            non_system.pop(0)
            compressed = system + non_system
        
        return None
    
    def _level2_keep_recent(self, messages: List[Message], target: int) -> Tuple[List[Message], int]:
        """Level 2: Keep system + last N messages"""
        system = [m for m in messages if m.role == "system"]
        non_system = [m for m in messages if m.role != "system"]
        
        for keep_count in range(len(non_system), 0, -1):
            compressed = system + non_system[-keep_count:]
            tokens = self.counter.count_messages_tokens(compressed)
            if tokens <= target:
                return compressed, 2
        
        return None


_compressor_cache = {}


def get_message_compressor(model_name: str = "gpt-4") -> MessageCompressor:
    """Get or create compressor instance (cached)"""
    if model_name not in _compressor_cache:
        _compressor_cache[model_name] = MessageCompressor(model_name)
    return _compressor_cache[model_name]