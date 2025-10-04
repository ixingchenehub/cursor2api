#!/usr/bin/env python3
"""
API Endpoints Testing Script for CursorWeb2API
This script tests all 4 API endpoints exposed by the service
"""

import requests
import json
import sys
import os
from typing import Dict, Any
from pathlib import Path

# Load API_KEY from .env file
def load_api_key() -> str:
    """Load API_KEY from .env file in current directory"""
    env_file = Path(__file__).parent / ".env"
    
    if not env_file.exists():
        print(f"ERROR: .env file not found at {env_file}")
        print("Please ensure .env file exists in the project root directory")
        sys.exit(1)
    
    with open(env_file, 'r', encoding='utf-8') as f:
        for line in f:
            line = line.strip()
            # Skip comments and empty lines
            if line.startswith('#') or not line or '=' not in line:
                continue
            # Parse KEY=VALUE
            if line.startswith('API_KEY='):
                key, _, value = line.partition('=')
                return value.strip()
    
    print("ERROR: API_KEY not found in .env file")
    sys.exit(1)

# Configuration
BASE_URL = "http://localhost:8000"
API_KEY = load_api_key()

print(f"Loaded API_KEY: {API_KEY[:20]}..." if len(API_KEY) > 20 else f"Loaded API_KEY: {API_KEY}")

# Headers with authentication
HEADERS = {
    "Authorization": f"Bearer {API_KEY}",
    "Content-Type": "application/json"
}


def print_section(title: str):
    """Print formatted section header"""
    print("\n" + "=" * 80)
    print(f"  {title}")
    print("=" * 80)


def print_result(success: bool, endpoint: str, details: str = ""):
    """Print test result"""
    status = "✓ PASS" if success else "✗ FAIL"
    print(f"{status} | {endpoint}")
    if details:
        print(f"     {details}")


def test_chat_completions() -> bool:
    """Test POST /v1/chat/completions endpoint"""
    print_section("Test 1/4: POST /v1/chat/completions")
    
    try:
        payload = {
            "model": "anthropic/claude-3-5-haiku-20241022",
            "messages": [
                {"role": "user", "content": "Say 'Hello, API test successful!' in one sentence."}
            ],
            "stream": False
        }
        
        print(f"Request: POST {BASE_URL}/v1/chat/completions")
        print(f"Payload: {json.dumps(payload, ensure_ascii=False)[:100]}...")
        
        response = requests.post(
            f"{BASE_URL}/v1/chat/completions",
            headers=HEADERS,
            json=payload,
            timeout=30
        )
        
        print(f"Status Code: {response.status_code}")
        
        if response.status_code == 200:
            data = response.json()
            print(f"Response Preview: {json.dumps(data, ensure_ascii=False)[:200]}...")
            
            # Validate response structure
            if "choices" in data and len(data["choices"]) > 0:
                content = data["choices"][0].get("message", {}).get("content", "")
                print(f"AI Response: {content[:100]}...")
                print_result(True, "/v1/chat/completions", "Response received successfully")
                return True
            else:
                print_result(False, "/v1/chat/completions", "Invalid response structure")
                return False
        else:
            print(f"Error: {response.text[:200]}")
            print_result(False, "/v1/chat/completions", f"HTTP {response.status_code}")
            return False
            
    except Exception as e:
        print(f"Exception: {str(e)}")
        print_result(False, "/v1/chat/completions", str(e))
        return False


def test_fingerprint_generate() -> bool:
    """Test POST /v1/fingerprint/generate endpoint"""
    print_section("Test 2/4: POST /v1/fingerprint/generate")
    
    try:
        payload = {
            "mode": "current"
        }
        
        print(f"Request: POST {BASE_URL}/v1/fingerprint/generate")
        print(f"Payload: {json.dumps(payload)}")
        
        response = requests.post(
            f"{BASE_URL}/v1/fingerprint/generate",
            headers=HEADERS,
            json=payload,
            timeout=10
        )
        
        print(f"Status Code: {response.status_code}")
        
        if response.status_code == 200:
            data = response.json()
            print(f"Response Preview: {json.dumps(data, ensure_ascii=False)[:200]}...")
            
            # Validate response structure
            if "fingerprint" in data and "base64" in data:
                print(f"Fingerprint keys: {list(data['fingerprint'].keys())}")
                print(f"Base64 length: {len(data['base64'])}")
                print_result(True, "/v1/fingerprint/generate", "Fingerprint generated successfully")
                return True
            else:
                print_result(False, "/v1/fingerprint/generate", "Invalid response structure")
                return False
        else:
            print(f"Error: {response.text[:200]}")
            print_result(False, "/v1/fingerprint/generate", f"HTTP {response.status_code}")
            return False
            
    except Exception as e:
        print(f"Exception: {str(e)}")
        print_result(False, "/v1/fingerprint/generate", str(e))
        return False


def test_list_models() -> bool:
    """Test GET /v1/models endpoint"""
    print_section("Test 3/4: GET /v1/models")
    
    try:
        print(f"Request: GET {BASE_URL}/v1/models")
        
        response = requests.get(
            f"{BASE_URL}/v1/models",
            headers=HEADERS,
            timeout=10
        )
        
        print(f"Status Code: {response.status_code}")
        
        if response.status_code == 200:
            data = response.json()
            print(f"Response: {json.dumps(data, ensure_ascii=False, indent=2)}")
            
            # Validate response structure
            if "object" in data and "data" in data and isinstance(data["data"], list):
                model_count = len(data["data"])
                model_ids = [m.get("id") for m in data["data"]]
                print(f"Total models: {model_count}")
                print(f"Model IDs: {', '.join(model_ids[:3])}...")
                print_result(True, "/v1/models", f"{model_count} models available")
                return True
            else:
                print_result(False, "/v1/models", "Invalid response structure")
                return False
        else:
            print(f"Error: {response.text[:200]}")
            print_result(False, "/v1/models", f"HTTP {response.status_code}")
            return False
            
    except Exception as e:
        print(f"Exception: {str(e)}")
        print_result(False, "/v1/models", str(e))
        return False


def test_resilience_metrics() -> bool:
    """Test GET /metrics/resilience endpoint"""
    print_section("Test 4/4: GET /metrics/resilience")
    
    try:
        print(f"Request: GET {BASE_URL}/metrics/resilience")
        
        response = requests.get(
            f"{BASE_URL}/metrics/resilience",
            headers=HEADERS,
            timeout=10
        )
        
        print(f"Status Code: {response.status_code}")
        
        if response.status_code == 200:
            data = response.json()
            print(f"Response: {json.dumps(data, ensure_ascii=False, indent=2)}")
            
            # Validate response structure - updated to match actual API response
            required_fields = ["total_requests", "successful_requests", "failed_requests",
                             "success_rate", "retries_count", "circuit_breaker_state",
                             "rate_limiter_tokens"]
            
            if all(field in data for field in required_fields):
                print_result(True, "/metrics/resilience", "Metrics retrieved successfully")
                return True
            else:
                missing = [f for f in required_fields if f not in data]
                print_result(False, "/metrics/resilience", f"Missing fields: {missing}")
                return False
        else:
            print(f"Error: {response.text[:200]}")
            print_result(False, "/metrics/resilience", f"HTTP {response.status_code}")
            return False
            
    except Exception as e:
        print(f"Exception: {str(e)}")
        print_result(False, "/metrics/resilience", str(e))
        return False


def main():
    """Main test runner"""
    print("\n" + "=" * 80)
    print("  CursorWeb2API - API Endpoints Testing")
    print("  Base URL: " + BASE_URL)
    print("=" * 80)
    
    results = []
    
    # Run all tests
    results.append(("Chat Completions", test_chat_completions()))
    results.append(("Fingerprint Generate", test_fingerprint_generate()))
    results.append(("List Models", test_list_models()))
    results.append(("Resilience Metrics", test_resilience_metrics()))
    
    # Summary
    print_section("Test Summary")
    passed = sum(1 for _, result in results if result)
    total = len(results)
    
    for name, result in results:
        status = "✓ PASS" if result else "✗ FAIL"
        print(f"{status} | {name}")
    
    print(f"\nTotal: {passed}/{total} tests passed")
    print("=" * 80)
    
    # Exit code
    sys.exit(0 if passed == total else 1)


if __name__ == "__main__":
    main()