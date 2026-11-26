from __future__ import annotations

import base64
import io
import json
import logging
import time
from dataclasses import dataclass
from typing import Any

import httpx
from PIL import Image, ImageOps

from ..settings import settings

logger = logging.getLogger("goodser-vision.vlm")


@dataclass
class VLMStatus:
    provider: str
    model: str
    enabled: bool
    loaded: bool
    last_used: float


class VLM:
    """VLM 复核。

    支持 OpenAI 兼容的 /chat/completions 视觉接口：
      - vLLM / Ollama / Qwen-VL 本地服务
      - 云厂商或订阅网关（需确认其提供 OpenAI 兼容 API 与图片输入权限）

    不在本进程加载 7B 权重；本地模型建议用 vLLM/Ollama 暴露 OpenAI 兼容接口。
    """

    def __init__(self) -> None:
        self.provider = settings.vlm_provider if settings.vlm_enabled else "disabled"
        self.model = settings.vlm_model
        self._last_used = 0.0

    @property
    def enabled(self) -> bool:
        return self.provider not in {"", "disabled", "off", "none"}

    @property
    def loaded(self) -> bool:
        # 云端/外部 OpenAI 兼容服务不在本进程持有权重
        return self.enabled and bool(settings.vlm_base_url)

    def load(self) -> None:
        # 外部服务无需加载；本地 vLLM/Ollama 由外部容器管理
        return

    def unload(self) -> None:
        return

    def maybe_unload(self, idle_seconds: int) -> None:
        return

    def status(self) -> VLMStatus:
        return VLMStatus(
            provider=self.provider,
            model=self.model,
            enabled=self.enabled,
            loaded=self.loaded,
            last_used=self._last_used,
        )

    def verify(self, image: Image.Image, prompt: str, candidates: list[dict[str, Any]] | None = None) -> str:
        if not self.enabled:
            raise RuntimeError("VLM 未启用")
        if not settings.vlm_base_url:
            raise RuntimeError("GOODSER_VISION_VLM_BASE_URL 未配置")
        self._last_used = time.monotonic()
        content = prompt
        if candidates:
            content += "\n候选商品：\n" + json.dumps(candidates, ensure_ascii=False)
        data_url = self._data_url(image)
        payload = {
            "model": self.model,
            "messages": [
                {
                    "role": "user",
                    "content": [
                        {"type": "text", "text": content},
                        {"type": "image_url", "image_url": {"url": data_url}},
                    ],
                }
            ],
            "max_tokens": settings.vlm_max_tokens,
            "temperature": 0.1,
        }
        headers = {"Content-Type": "application/json"}
        if settings.vlm_api_key:
            headers["Authorization"] = f"Bearer {settings.vlm_api_key}"
        url = settings.vlm_base_url.rstrip("/") + "/chat/completions"
        with httpx.Client(timeout=settings.vlm_timeout_seconds) as client:
            resp = client.post(url, headers=headers, json=payload)
        if resp.status_code < 200 or resp.status_code >= 300:
            raise RuntimeError(f"VLM 返回 {resp.status_code}: {resp.text[:500]}")
        body = resp.json()
        choices = body.get("choices") or []
        if not choices:
            raise RuntimeError("VLM 响应为空")
        message = choices[0].get("message") or {}
        return str(message.get("content") or "")

    def _data_url(self, image: Image.Image) -> str:
        img = ImageOps.exif_transpose(image).convert("RGB")
        if max(img.size) > settings.vlm_max_image_side:
            img.thumbnail(
                (settings.vlm_max_image_side, settings.vlm_max_image_side),
                Image.Resampling.LANCZOS,
            )
        buf = io.BytesIO()
        img.save(buf, format="JPEG", quality=88)
        encoded = base64.b64encode(buf.getvalue()).decode("ascii")
        return f"data:image/jpeg;base64,{encoded}"
