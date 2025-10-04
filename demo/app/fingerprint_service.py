"""
Browser Fingerprint Generation Service

This module provides comprehensive browser fingerprint generation capabilities
with support for multiple generation modes (current, desktop, mobile, any).
"""

import json
import random
import base64
from typing import Dict, List, Any, Literal


class FingerprintDatabase:
    """Database of browser fingerprint templates organized by platform and browser"""
    
    DESKTOP = {
        "windows": {
            "chrome": [
                {
                    "userAgent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
                    "platform": "Win32",
                    "gpuVendor": "Google Inc. (Intel)",
                    "gpuRenderer": "ANGLE (Intel, Intel(R) UHD Graphics 630 Direct3D11 vs_5_0 ps_5_0, D3D11)",
                    "screenResolution": ["1920x1080", "2560x1440", "1366x768"],
                    "languages": ["en-US", "en", "zh-CN"],
                    "hardwareConcurrency": [8, 16, 4],
                    "deviceMemory": [8, 16, 4],
                    "maxTouchPoints": 0
                },
                {
                    "userAgent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
                    "platform": "Win32",
                    "gpuVendor": "Google Inc. (NVIDIA)",
                    "gpuRenderer": "ANGLE (NVIDIA, NVIDIA GeForce RTX 3060 Direct3D11 vs_5_0 ps_5_0, D3D11)",
                    "screenResolution": ["1920x1080", "2560x1440", "3840x2160"],
                    "languages": ["en-US", "en"],
                    "hardwareConcurrency": [12, 16, 8],
                    "deviceMemory": [16, 32, 8],
                    "maxTouchPoints": 0
                }
            ],
            "firefox": [
                {
                    "userAgent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
                    "platform": "Win32",
                    "gpuVendor": "Intel Inc.",
                    "gpuRenderer": "Intel(R) UHD Graphics 630",
                    "screenResolution": ["1920x1080", "2560x1440", "1366x768"],
                    "languages": ["en-US", "en", "zh-CN"],
                    "hardwareConcurrency": [8, 16, 4],
                    "deviceMemory": [8, 16, 4],
                    "maxTouchPoints": 0
                }
            ],
            "edge": [
                {
                    "userAgent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
                    "platform": "Win32",
                    "gpuVendor": "Google Inc. (AMD)",
                    "gpuRenderer": "ANGLE (AMD, AMD Radeon RX 6700 XT Direct3D11 vs_5_0 ps_5_0, D3D11)",
                    "screenResolution": ["1920x1080", "2560x1440"],
                    "languages": ["en-US", "en"],
                    "hardwareConcurrency": [12, 16],
                    "deviceMemory": [16, 32],
                    "maxTouchPoints": 0
                }
            ]
        },
        "macos": {
            "chrome": [
                {
                    "userAgent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
                    "platform": "MacIntel",
                    "gpuVendor": "Apple Inc.",
                    "gpuRenderer": "Apple M1",
                    "screenResolution": ["2560x1600", "1920x1200", "1440x900"],
                    "languages": ["en-US", "en"],
                    "hardwareConcurrency": [8, 10],
                    "deviceMemory": [8, 16],
                    "maxTouchPoints": 0
                }
            ],
            "safari": [
                {
                    "userAgent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Safari/605.1.15",
                    "platform": "MacIntel",
                    "gpuVendor": "Apple Inc.",
                    "gpuRenderer": "Apple M2",
                    "screenResolution": ["2560x1600", "1920x1200"],
                    "languages": ["en-US", "en"],
                    "hardwareConcurrency": [8, 10],
                    "deviceMemory": [8, 16],
                    "maxTouchPoints": 0
                }
            ]
        },
        "linux": {
            "chrome": [
                {
                    "userAgent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
                    "platform": "Linux x86_64",
                    "gpuVendor": "Intel Inc.",
                    "gpuRenderer": "Mesa Intel(R) UHD Graphics 630 (CML GT2)",
                    "screenResolution": ["1920x1080", "2560x1440"],
                    "languages": ["en-US", "en"],
                    "hardwareConcurrency": [8, 16],
                    "deviceMemory": [8, 16],
                    "maxTouchPoints": 0
                }
            ],
            "firefox": [
                {
                    "userAgent": "Mozilla/5.0 (X11; Linux x86_64; rv:121.0) Gecko/20100101 Firefox/121.0",
                    "platform": "Linux x86_64",
                    "gpuVendor": "Intel Inc.",
                    "gpuRenderer": "Mesa Intel(R) UHD Graphics 630",
                    "screenResolution": ["1920x1080", "2560x1440"],
                    "languages": ["en-US", "en"],
                    "hardwareConcurrency": [8, 16],
                    "deviceMemory": [8, 16],
                    "maxTouchPoints": 0
                }
            ]
        }
    }
    
    MOBILE = {
        "ios": {
            "safari": [
                {
                    "userAgent": "Mozilla/5.0 (iPhone; CPU iPhone OS 17_1_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Mobile/15E148 Safari/604.1",
                    "platform": "iPhone",
                    "gpuVendor": "Apple Inc.",
                    "gpuRenderer": "Apple A16 GPU",
                    "screenResolution": ["390x844", "428x926", "375x812"],
                    "languages": ["en-US", "en"],
                    "hardwareConcurrency": [6],
                    "deviceMemory": [4, 6],
                    "maxTouchPoints": 5
                }
            ],
            "chrome": [
                {
                    "userAgent": "Mozilla/5.0 (iPhone; CPU iPhone OS 17_1_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/120.0.6099.119 Mobile/15E148 Safari/604.1",
                    "platform": "iPhone",
                    "gpuVendor": "Apple Inc.",
                    "gpuRenderer": "Apple A15 GPU",
                    "screenResolution": ["390x844", "428x926"],
                    "languages": ["en-US", "en"],
                    "hardwareConcurrency": [6],
                    "deviceMemory": [4, 6],
                    "maxTouchPoints": 5
                }
            ]
        },
        "android": {
            "chrome": [
                {
                    "userAgent": "Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.6099.144 Mobile Safari/537.36",
                    "platform": "Linux armv8l",
                    "gpuVendor": "Qualcomm",
                    "gpuRenderer": "Adreno (TM) 740",
                    "screenResolution": ["360x800", "412x915", "393x873"],
                    "languages": ["en-US", "en"],
                    "hardwareConcurrency": [8],
                    "deviceMemory": [6, 8],
                    "maxTouchPoints": 5
                }
            ]
        }
    }


class FingerprintGenerator:
    """
    Main fingerprint generation service with multiple generation modes.
    
    Supports:
    - current: Generate based on environment FP configuration
    - desktop: Random desktop browser fingerprint
    - mobile: Random mobile browser fingerprint
    - any: Random from all available fingerprints
    """
    
    def __init__(self, env_fp: Dict[str, Any] = None):
        """
        Initialize the fingerprint generator.
        
        Args:
            env_fp: The FP configuration from environment variables
        """
        self.env_fp = env_fp or {}
        self.db = FingerprintDatabase()
    
    def generate(
        self, 
        mode: Literal["current", "desktop", "mobile", "any"] = "current"
    ) -> Dict[str, Any]:
        """
        Generate a browser fingerprint based on the specified mode.
        
        Args:
            mode: Generation mode (current, desktop, mobile, any)
            
        Returns:
            Complete fingerprint dictionary with all properties
        """
        if mode == "current":
            return self._generate_from_env()
        elif mode == "desktop":
            return self._generate_random_desktop()
        elif mode == "mobile":
            return self._generate_random_mobile()
        elif mode == "any":
            return self._generate_random_any()
        else:
            raise ValueError(f"Invalid mode: {mode}. Must be one of: current, desktop, mobile, any")
    
    def _generate_from_env(self) -> Dict[str, Any]:
        """
        Generate fingerprint from environment FP configuration.
        
        Returns:
            Fingerprint based on current environment configuration
        """
        if not self.env_fp:
            # Fallback to random desktop if no env FP configured
            return self._generate_random_desktop()
        
        # Return the environment FP as-is if it has all required fields
        return self.env_fp
    
    def _generate_random_desktop(self) -> Dict[str, Any]:
        """
        Generate a random desktop browser fingerprint.
        
        Returns:
            Random desktop fingerprint
        """
        desktop_pool = []
        for os_name in self.db.DESKTOP:
            for browser in self.db.DESKTOP[os_name]:
                desktop_pool.extend(self.db.DESKTOP[os_name][browser])
        
        template = random.choice(desktop_pool)
        return self._build_fingerprint(template)
    
    def _generate_random_mobile(self) -> Dict[str, Any]:
        """
        Generate a random mobile browser fingerprint.
        
        Returns:
            Random mobile fingerprint
        """
        mobile_pool = []
        for os_name in self.db.MOBILE:
            for browser in self.db.MOBILE[os_name]:
                mobile_pool.extend(self.db.MOBILE[os_name][browser])
        
        template = random.choice(mobile_pool)
        return self._build_fingerprint(template)
    
    def _generate_random_any(self) -> Dict[str, Any]:
        """
        Generate a random fingerprint from all available templates.
        
        Returns:
            Random fingerprint from desktop or mobile
        """
        all_pool = []
        
        # Add all desktop fingerprints
        for os_name in self.db.DESKTOP:
            for browser in self.db.DESKTOP[os_name]:
                all_pool.extend(self.db.DESKTOP[os_name][browser])
        
        # Add all mobile fingerprints
        for os_name in self.db.MOBILE:
            for browser in self.db.MOBILE[os_name]:
                all_pool.extend(self.db.MOBILE[os_name][browser])
        
        template = random.choice(all_pool)
        return self._build_fingerprint(template)
    
    def _build_fingerprint(self, template: Dict[str, Any]) -> Dict[str, Any]:
        """
        Build a complete fingerprint from a template with randomized values.
        
        Args:
            template: Fingerprint template with value arrays
            
        Returns:
            Complete fingerprint with all properties
        """
        # Randomly select values from arrays
        hardware_concurrency = random.choice(template["hardwareConcurrency"])
        device_memory = random.choice(template["deviceMemory"])
        screen_resolution = random.choice(template["screenResolution"])
        
        # Determine pixel ratio based on device type
        is_mobile = template["maxTouchPoints"] > 0
        pixel_ratio = random.choice([2, 3]) if is_mobile else 1
        
        # Build the complete fingerprint
        fingerprint = {
            "userAgent": template["userAgent"],
            "platform": template["platform"],
            "language": template["languages"][0],
            "languages": template["languages"],
            "hardwareConcurrency": hardware_concurrency,
            "deviceMemory": device_memory,
            "maxTouchPoints": template["maxTouchPoints"],
            "screenResolution": screen_resolution,
            "colorDepth": 24,
            "pixelRatio": pixel_ratio,
            "timezone": "Asia/Shanghai",
            "timezoneOffset": -480,
            "webgl": {
                "vendor": template["gpuVendor"],
                "renderer": template["gpuRenderer"]
            },
            "canvas": self._generate_canvas_fingerprint(template["gpuRenderer"]),
            "audio": self._generate_audio_fingerprint()
        }
        
        return fingerprint
    
    def _generate_canvas_fingerprint(self, seed: str = "") -> str:
        """
        Generate a deterministic canvas fingerprint based on GPU renderer.
        
        Args:
            seed: GPU renderer string to use as seed
            
        Returns:
            Canvas fingerprint hash (60 characters)
        """
        # Generate a pseudo-random but consistent hash based on the seed
        # This simulates the canvas rendering fingerprint
        import hashlib
        hash_input = f"BrowserFP{seed}".encode('utf-8')
        hash_result = hashlib.sha256(hash_input).hexdigest()
        return hash_result[:60]
    
    def _generate_audio_fingerprint(self) -> str:
        """
        Generate a random audio context fingerprint.
        
        Returns:
            Audio fingerprint value (10 decimal places)
        """
        # Generate a small random float to simulate audio context
        # Generate a small random float to simulate audio context fingerprint
        audio_value = random.uniform(0.0000000001, 0.0000001000)
        return f"{audio_value:.10f}"


def encode_fingerprint_to_base64(fingerprint: Dict[str, Any]) -> str:
    """
    Encode a fingerprint dictionary to Base64 string.
    
    Args:
        fingerprint: Complete fingerprint dictionary
        
    Returns:
        Base64 encoded string
    """
    json_string = json.dumps(fingerprint, ensure_ascii=False, separators=(',', ':'))
    return base64.b64encode(json_string.encode('utf-8')).decode('utf-8')


def decode_base64_to_fingerprint(base64_string: str) -> Dict[str, Any]:
    """
    Decode a Base64 string back to fingerprint dictionary.
    
    Args:
        base64_string: Base64 encoded fingerprint string
        
    Returns:
        Decoded fingerprint dictionary
        
    Raises:
        ValueError: If the base64 string is invalid
    """
    try:
        json_bytes = base64.b64decode(base64_string)
        json_string = json_bytes.decode('utf-8')
        return json.loads(json_string)
    except Exception as e:
        raise ValueError(f"Invalid base64 fingerprint string: {str(e)}")