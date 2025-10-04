import asyncio
import base64
import json
import random
import string
import time
import uuid
from functools import wraps
from typing import Union, Callable, Any, AsyncGenerator, Dict

from curl_cffi.requests.exceptions import RequestException
from sse_starlette import EventSourceResponse
from starlette.responses import JSONResponse

from app.errors import CursorWebError
from app.models import ChatCompletionRequest, Usage, ToolCall


async def safe_stream_wrapper(
        generator_func, *args, **kwargs
) -> Union[EventSourceResponse, JSONResponse]:
    """
    安全的流响应包装器
    先执行生成器获取第一个值，如果成功才创建流响应
    """
    # 创建生成器实例
    generator = generator_func(*args, **kwargs)

    # 尝试获取第一个值
    first_item = await generator.__anext__()

    # 如果成功获取第一个值，创建新的生成器包装原生成器
    async def wrapped_generator():
        # 先yield第一个值
        yield first_item
        # 然后yield剩余的值
        async for item in generator:
            yield item

    # 创建流响应
    return EventSourceResponse(
        wrapped_generator(),
        media_type="text/event-stream",
        headers={
            "Cache-Control": "no-cache",
            "Connection": "keep-alive",
            "X-Accel-Buffering": "no",
        },
    )


async def error_wrapper(func: Callable, *args, **kwargs) -> Any:
    from .config import MAX_RETRIES
    for attempt in range(MAX_RETRIES + 1):  # 包含初始尝试，所以是 MAX_RETRIES + 1
        try:
            return await func(*args, **kwargs)
        except (CursorWebError, RequestException) as e:

            # 如果已经达到最大重试次数，返回错误响应
            if attempt == MAX_RETRIES:
                if isinstance(e, CursorWebError):
                    return JSONResponse(
                        e.to_openai_error(),
                        status_code=e.response_status_code
                    )
                elif isinstance(e, RequestException):
                    return JSONResponse(
                        {
                            'error': {
                                'message': str(e),
                                "type": "http_error",
                                "code": "http_error"
                            }
                        },
                        status_code=500
                    )

            if attempt < MAX_RETRIES:
                continue
    return None


def decode_base64url_safe(data):
    """使用安全的base64url解码"""
    # 添加必要的填充
    missing_padding = len(data) % 4
    if missing_padding:
        data += '=' * (4 - missing_padding)

    return base64.urlsafe_b64decode(data)


def to_async(sync_func):
    @wraps(sync_func)
    async def async_wrapper(*args):
        loop = asyncio.get_running_loop()
        return await loop.run_in_executor(None, sync_func, *args)

    return async_wrapper


def generate_random_string(length):
    """
    生成一个指定长度的随机字符串，包含大小写字母和数字。
    """
    # 定义所有可能的字符：大小写字母和数字
    characters = string.ascii_letters + string.digits

    # 使用 random.choice 从字符集中随机选择字符，重复 length 次，然后拼接起来
    random_string = ''.join(random.choice(characters) for _ in range(length))
    return random_string


def normalize_tool_name(name: str) -> str:
    """将工具名统一标准化：将所有下划线替换为连字符"""
    return name.replace('_', '-')


def match_tool_name(tool_name: str, available_tools: list[str]) -> str:
    """
    匹配工具名称，如果不在列表中则尝试标准化匹配

    Args:
        tool_name: 需要匹配的工具名
        available_tools: 可用的工具名列表

    Returns:
        匹配到的实际工具名，如果没有匹配返回原名称
    """
    # 直接匹配
    if tool_name in available_tools:
        return tool_name

    # 标准化后匹配
    normalized_input = normalize_tool_name(tool_name)
    for available_tool in available_tools:
        if normalize_tool_name(available_tool) == normalized_input:
            return available_tool

    # 没有匹配，返回原名称
    return tool_name


async def non_stream_chat_completion(
        request: ChatCompletionRequest,
        generator: AsyncGenerator[str, None]
) -> Dict[str, Any]:
    """
    非流式响应：接受外部异步生成器，收集所有输出返回完整响应
    """
    # 收集所有流式输出
    full_content = ""
    tool_calls = []
    usage = Usage(prompt_tokens=0, completion_tokens=0, total_tokens=0)
    async for chunk in generator:
        if isinstance(chunk, Usage):
            usage = chunk
            continue
        if isinstance(chunk, ToolCall):
            tool_calls.append({
                "id": chunk.toolId,
                "type": "function",
                "function": {
                    "name": chunk.toolName,
                    "arguments": chunk.toolInput,
                }
            })
            continue
        full_content += chunk

    # 构造OpenAI格式的响应
    response = {
        "id": f"chatcmpl-{uuid.uuid4().hex[:29]}",
        "object": "chat.completion",
        "created": int(time.time()),
        "model": request.model,
        "choices": [
            {
                "index": 0,
                "message": {
                    "role": "assistant",
                    "content": full_content,
                    "tool_calls":tool_calls
                },
                "finish_reason": "stop"
            }
        ],
        "usage": {
            "prompt_tokens": usage.prompt_tokens,
            "completion_tokens": usage.completion_tokens,
            "total_tokens": usage.total_tokens
        }
    }

    return response


async def stream_chat_completion(
        request: ChatCompletionRequest,
        generator: AsyncGenerator[str, None]
) -> AsyncGenerator[Dict[str, Any], None]:
    """
    流式响应：接受外部异步生成器，包装成OpenAI SSE格式
    """
    chat_id = f"chatcmpl-{uuid.uuid4().hex[:29]}"
    created_time = int(time.time())

    is_send_init = False

    # 发送初始流式响应头
    initial_response = {
        "id": chat_id,
        "object": "chat.completion.chunk",
        "created": created_time,
        "model": request.model,
        "choices": [
            {
                "index": 0,
                "delta": {"role": "assistant", "content": ""},
                "finish_reason": None
            }
        ]
    }

    # 流式发送内容
    usage = None
    tool_call_idx = 0
    async for chunk in generator:
        if not is_send_init:
            yield {
                "data": json.dumps(initial_response, ensure_ascii=False)
            }
            is_send_init = True
        if isinstance(chunk, Usage):
            usage = chunk
            continue

        if isinstance(chunk, ToolCall):
            data = {
                "id": chat_id,
                "object": "chat.completion.chunk",
                "created": created_time,
                "model": request.model,
                "choices": [
                    {
                        "index": 0,
                        "delta": {
                            "tool_calls": [
                                {
                                    "index": tool_call_idx,
                                    "id": chunk.toolId,
                                    "type": "function",
                                    "function": {
                                        "name": chunk.toolName,
                                        "arguments": chunk.toolInput,
                                    },
                                }
                            ]
                        },
                        "finish_reason": None,
                    }
                ],
            }
            tool_call_idx += 1
            yield {'data': json.dumps(data, ensure_ascii=False)}
            continue


        chunk_response = {
            "id": chat_id,
            "object": "chat.completion.chunk",
            "created": created_time,
            "model": request.model,
            "choices": [
                {
                    "index": 0,
                    "delta": {"content": chunk},
                    "finish_reason": None
                }
            ]
        }
        yield {"data": json.dumps(chunk_response, ensure_ascii=False)}

    # 发送结束标记
    final_response = {
        "id": chat_id,
        "object": "chat.completion.chunk",
        "created": created_time,
        "model": request.model,
        "choices": [
            {
                "index": 0,
                "delta": {},
                "finish_reason": "stop"
            }
        ]
    }
    yield {"data": json.dumps(final_response, ensure_ascii=False)}
    if usage:
        usage_data = {"id": chat_id, "object": "chat.completion.chunk",
                      "created": created_time, "model": request.model,
                      "choices": [],
                      "usage": {"prompt_tokens": usage.prompt_tokens,
                                "completion_tokens": usage.completion_tokens,
                                "total_tokens": usage.total_tokens, "prompt_tokens_details": {
                              "cached_tokens": 0,
                              "text_tokens": 0,
                              "audio_tokens": 0,
                              "image_tokens": 0
                          },
                                "completion_tokens_details": {
                                    "text_tokens": 0,
                                    "audio_tokens": 0,
                                    "reasoning_tokens": 0
                                },
                                "input_tokens": 0,
                                "output_tokens": 0,
                                "input_tokens_details": None}
                      }

        yield {
            "data": json.dumps(usage_data, ensure_ascii=False)
        }
    yield {"data": "[DONE]"}
