from typing import List, Dict, Any, Literal, Optional

from pydantic import BaseModel, Field


class OpenAIToolCallFunction(BaseModel):
    """工具调用函数"""

    name: str | None = Field(None, description="函数名称")
    arguments: str | None = Field(None, description="JSON格式的函数参数")


class OpenAIDeltaToolCall(BaseModel):
    index: int | None = Field(None, description="工具调用索引")
    id: str | None = Field(None, description="工具调用ID")
    type: Literal["function"] | None = Field(None, description="调用类型")
    function: OpenAIToolCallFunction | None = Field(None, description="函数详情增量")


class OpenAIMessageContent(BaseModel):
    """OpenAI消息内容项"""

    type: Literal["text", "image_url"] = Field(description="内容类型")
    text: str | None = Field(None, description="文本内容")
    image_url: dict[str, str] | None = Field(None, description="图像URL配置")


class Message(BaseModel):
    role: str
    content: str | list[OpenAIMessageContent] | None = Field(
        None, description="消息内容"
    )
    tool_call_id: str | None = Field(None)
    tool_calls: list[dict[str, Any]] | None = Field(
        None, description="工具调用信息（当role为assistant时）"
    )


class OpenAIToolFunction(BaseModel):
    """OpenAI工具函数定义"""

    name: str = Field(description="函数名称")
    description: str | None = Field(None, description="函数描述")
    parameters: dict[str, Any] | None = Field(
        None, description="JSON Schema格式的函数参数"
    )


class OpenAITool(BaseModel):
    """OpenAI工具定义"""

    type: Literal["function"] = Field("function", description="工具类型")
    function: OpenAIToolFunction = Field(description="函数定义")


class ChatCompletionRequest(BaseModel):
    messages: List[Message]
    stream: Optional[bool] = False
    model: Optional[str] = "gpt-4o"
    tools: list[OpenAITool] | None = Field(None, description="可用工具定义")


class Model(BaseModel):
    id: str
    object: str
    created: int
    owned_by: str


class ModelsResponse(BaseModel):
    object: str
    data: List[Model]


class Choice(BaseModel):
    index: int
    message: Optional[Dict[str, Any]] = None
    delta: Optional[Dict[str, Any]] = None
    finish_reason: Optional[str] = None


class Usage(BaseModel):
    prompt_tokens: int
    completion_tokens: int
    total_tokens: int


class ChatCompletionResponse(BaseModel):
    id: str
    object: str
    created: int
    model: str
    choices: List[Choice]
    usage: Optional[Usage] = None


class ToolCall(BaseModel):
    toolName: str
    toolId: str
    toolInput: str



class FingerprintRequest(BaseModel):
    """Browser fingerprint generation request model"""

    mode: Literal["current", "desktop", "mobile", "any"] = Field(
        default="current",
        description="指纹生成模式: current(使用环境变量FP配置), desktop(随机桌面端指纹), mobile(随机移动端指纹), any(从所有指纹中随机选择)"
    )


class FingerprintResponse(BaseModel):
    """Browser fingerprint generation response model"""

    fingerprint: Dict[str, Any] = Field(
        ...,
        description="完整的浏览器指纹对象,包含userAgent、platform、webgl、canvas、audio、screen等属性"
    )
    base64: str = Field(
        ...,
        description="Base64编码的指纹字符串,可直接用于API调用"
    )