#!/usr/bin/env python3
"""
Load .env file into environment variables
This module must be imported before any config imports
"""
import os
from pathlib import Path


def load_dotenv():
    """Load .env file from project root into os.environ"""
    env_file = Path(__file__).parent / ".env"
    
    if not env_file.exists():
        print(f"WARNING: .env file not found at {env_file}")
        return
    
    with open(env_file, 'r', encoding='utf-8') as f:
        for line in f:
            line = line.strip()
            # Skip comments and empty lines
            if line.startswith('#') or not line or '=' not in line:
                continue
            
            # Parse KEY=VALUE
            key, _, value = line.partition('=')
            key = key.strip()
            value = value.strip()
            
            # Only set if not already in environment
            if key and key not in os.environ:
                os.environ[key] = value


# Auto-load when module is imported
load_dotenv()