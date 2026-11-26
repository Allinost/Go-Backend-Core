from __future__ import annotations

import base64
import io
import json
import logging
import os
import threading
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import httpx
from PIL import Image, ImageOps

from ..settings import settings

logger = logging.getLogger("goodser-vision.vlm")


@dataclass
class VLMStatus:
    provider: str
    model: str
    base_url: str
    enabled: bool
    configured: bool
    api_key_set: bool
    loaded: bool
    last_used: float


class VLM:
    """VLM 复核，通过 OpenAI 兼容接口接入 New API。

    New API（one-api fork）统一接入 opencode / commandcode / 本地 vLLM。
    不在本进程加载本地权重；API 不可用时直接返回错误，不回退本地模型。
    """

    def __init__(self) -> None:
        self._lock = threading.RLock()
        self.provider = self._normalize_provider(settings.vlm_provider)
        self.base_url = settings.vlm_base_url.rstrip("/")
        self.model = settings.vlm_model
        self.api_key = settings.vlm_api_key
        self._enabled_override: bool | None = None
        self._last_used = 0.0
        self._load_persisted()

    @staticmethod
    def _normalize_provider(value: str) -> str:
        provider = (value or "disabled").strip().lower()
        if provider in {"newapi", "new-api", "oneapi", "one-api"}:
            return "newapi"
        return provider or "disabled"

    @property
    def enabled(self) -> bool:
        base_enabled = self.provider not in {"", "disabled", "off", "none"}
        if self._enabled_override is not None:
            return self._enabled_override and base_enabled
        return base_enabled

    @property
    def configured(self) -> bool:
        return self.enabled and bool(self.base_url) and bool(self.model)

    @property
    def loaded(self) -> bool:
        # 外部 New API/OpenAI 兼容服务不在本进程持有权重
        return self.configured

    def set_enabled(self, enabled: bool) -> None:
        with self._lock:
            self._enabled_override = bool(enabled)
            if not enabled:
                self.unload()

    def update(
        self,
        enabled: bool | None = None,
        provider: str | None = None,
        base_url: str | None = None,
        model: str | None = None,
        api_key: str | None = None,
    ) -> dict[str, Any]:
        with self._lock:
            if provider is not None:
                self.provider = self._normalize_provider(provider)
            if base_url is not None:
                self.base_url = base_url.strip().rstrip("/")
            if model is not None:
                self.model = model.strip()
            if api_key is not None:
                self.api_key = api_key.strip()
            if enabled is not None:
                self._enabled_override = bool(enabled)
            self._persist()
            return self.status().__dict__

    def test(self) -> dict[str, Any]:
        if not self.configured:
            raise RuntimeError("VLM 未启用或缺少 BASE_URL / MODEL")
        headers = self._headers()
        url = self.base_url + "/models"
        with httpx.Client(timeout=settings.vlm_timeout_seconds) as client:
            resp = client.get(url, headers=headers)
        if resp.status_code < 200 or resp.status_code >= 300:
            raise RuntimeError(f"New API 返回 {resp.status_code}: {resp.text[:300]}")
        return {"ok": True, "base_url": self.base_url, "model": self.model}

    def load(self) -> None:
        # 外部服务无需加载
        return

    def unload(self) -> None:
        return

    def maybe_unload(self, idle_seconds: int) -> None:
        return

    def status(self) -> VLMStatus:
        return VLMStatus(
            provider=self.provider,
            model=self.model,
            base_url=self.base_url,
            enabled=self.enabled,
            configured=self.configured,
            api_key_set=bool(self.api_key),
            loaded=self.loaded,
            last_used=self._last_used,
        )

    def verify(self, image: Image.Image, prompt: str, candidates: list[dict[str, Any]] | None = None) -> str:
        if not self.enabled:
            raise RuntimeError("VLM 已关闭（不启用本地回退）")
        if not self.configured:
            raise RuntimeError("VLM 未配置：需要 base_url 与 model")
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
        url = self.base_url + "/chat/completions"
        with httpx.Client(timeout=settings.vlm_timeout_seconds) as client:
            resp = client.post(url, headers=self._headers(), json=payload)
        if resp.status_code < 200 or resp.status_code >= 300:
            raise RuntimeError(f"VLM 返回 {resp.status_code}: {resp.text[:500]}")
        body = resp.json()
        choices = body.get("choices") or []
        if not choices:
            raise RuntimeError("VLM 响应为空")
        message = choices[0].get("message") or {}
        return str(message.get("content") or "")

    def _headers(self) -> dict[str, str]:
        headers = {"Content-Type": "application/json"}
        if self.api_key:
            headers["Authorization"] = f"Bearer {self.api_key}"
        return headers

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

    def _load_persisted(self) -> None:
        path = Path(settings.vlm_config_file)
        if not path.is_absolute():
            path = Path.cwd() / path
        if not path.exists():
            return
        try:
            data = json.loads(path.read_text(encoding="utf-8"))
        except Exception as exc:  # noqa: BLE001
            logger.warning("load VLM config failed: %s", exc)
            return
        if "provider" in data:
            self.provider = self._normalize_provider(str(data["provider"]))
        if "base_url" in data:
            self.base_url = str(data["base_url"]).strip().rstrip("/")
        if "model" in data:
            self.model = str(data["model"]).strip()
        if "api_key" in data:
            self.api_key = str(data["api_key"]).strip()
        if "enabled" in data:
            self._enabled_override = bool(data["enabled"])

    def _persist(self) -> None:
        path = Path(settings.vlm_config_file)
        if not path.is_absolute():
            path = Path.cwd() / path
        path.parent.mkdir(parents=True, exist_ok=True)
        payload = {
            "provider": self.provider,
            "base_url": self.base_url,
            "model": self.model,
            "api_key": self.api_key,
            "enabled": self.enabled,
        }
        tmp = path.with_suffix(path.suffix + ".tmp")
        tmp.write_text(json.dumps(payload, ensure_ascii=False, indent=2), encoding="utf-8")
        try:
            os.chmod(tmp, 0o600)
        except OSError:
            pass
        os.replace(tmp, path)
