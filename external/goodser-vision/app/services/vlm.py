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


def _normalize_provider(value: str) -> str:
    provider = (value or "disabled").strip().lower()
    if provider in {"newapi", "new-api", "oneapi", "one-api"}:
        return "newapi"
    if provider in {"openai", "openai_compatible", "openai-compatible"}:
        return "openai"
    return provider or "disabled"


@dataclass
class VLMSlot:
    name: str
    enabled: bool
    provider: str
    base_url: str
    model: str
    api_key: str
    last_used: float = 0.0

    @property
    def protocol_enabled(self) -> bool:
        return self.provider not in {"", "disabled", "off", "none"}

    @property
    def configured(self) -> bool:
        return self.protocol_enabled and bool(self.base_url) and bool(self.model)

    def status(self) -> dict[str, Any]:
        return {
            "name": self.name,
            "enabled": self.enabled,
            "provider": self.provider,
            "base_url": self.base_url,
            "model": self.model,
            "configured": self.configured,
            "api_key_set": bool(self.api_key),
            "loaded": self.configured,
            "last_used": self.last_used,
        }


class VLM:
    """VLM 复核：New API 主/备双槽位。

    不在本进程加载本地权重；主失败时自动尝试备用，二者都不可用则返回错误。
    """

    def __init__(self) -> None:
        self._lock = threading.RLock()
        self._global_enabled = bool(settings.vlm_enabled)
        self.primary = VLMSlot(
            name="primary",
            enabled=bool(settings.vlm_primary_enabled),
            provider=_normalize_provider(settings.vlm_primary_provider),
            base_url=settings.vlm_primary_base_url.rstrip("/"),
            model=settings.vlm_primary_model,
            api_key=settings.vlm_primary_api_key,
        )
        self.backup = VLMSlot(
            name="backup",
            enabled=bool(settings.vlm_backup_enabled),
            provider=_normalize_provider(settings.vlm_backup_provider),
            base_url=settings.vlm_backup_base_url.rstrip("/"),
            model=settings.vlm_backup_model,
            api_key=settings.vlm_backup_api_key,
        )
        self._load_persisted()

    @property
    def enabled(self) -> bool:
        return self._global_enabled and any(
            slot.enabled and slot.protocol_enabled for slot in (self.primary, self.backup)
        )

    @property
    def configured(self) -> bool:
        return self._global_enabled and any(
            slot.enabled and slot.configured for slot in (self.primary, self.backup)
        )

    @property
    def loaded(self) -> bool:
        return self.configured

    def set_enabled(self, enabled: bool) -> None:
        with self._lock:
            self._global_enabled = bool(enabled)
            self._persist()

    def update(
        self,
        enabled: bool | None = None,
        primary: dict[str, Any] | None = None,
        backup: dict[str, Any] | None = None,
        provider: str | None = None,
        base_url: str | None = None,
        model: str | None = None,
        api_key: str | None = None,
    ) -> dict[str, Any]:
        with self._lock:
            if enabled is not None:
                self._global_enabled = bool(enabled)
            if any(value is not None for value in (provider, base_url, model, api_key)):
                self._apply_slot(self.primary, {
                    "provider": provider,
                    "base_url": base_url,
                    "model": model,
                    "api_key": api_key,
                })
            if primary:
                self._apply_slot(self.primary, primary)
            if backup:
                self._apply_slot(self.backup, backup)
            self._persist()
            return self.status()

    def _apply_slot(self, slot: VLMSlot, values: dict[str, Any]) -> None:
        if values.get("enabled") is not None:
            slot.enabled = bool(values["enabled"])
        if values.get("provider") is not None:
            slot.provider = _normalize_provider(str(values["provider"]))
        if values.get("base_url") is not None:
            slot.base_url = str(values["base_url"]).strip().rstrip("/")
        if values.get("model") is not None:
            slot.model = str(values["model"]).strip()
        if values.get("api_key") is not None:
            slot.api_key = str(values["api_key"]).strip()

    def _active_slots(self) -> list[VLMSlot]:
        if not self._global_enabled:
            return []
        return [
            slot
            for slot in (self.primary, self.backup)
            if slot.enabled and slot.configured
        ]

    def test(self, slot_name: str | None = None) -> dict[str, Any]:
        targets = [self.primary, self.backup]
        if slot_name:
            targets = [slot for slot in targets if slot.name == slot_name]
        out: dict[str, Any] = {}
        for slot in targets:
            if not slot.enabled or not slot.configured:
                out[slot.name] = {"ok": False, "error": "未启用或未配置"}
                continue
            try:
                headers = self._headers(slot)
                url = slot.base_url + "/models"
                with httpx.Client(timeout=settings.vlm_timeout_seconds) as client:
                    resp = client.get(url, headers=headers)
                if resp.status_code < 200 or resp.status_code >= 300:
                    raise RuntimeError(f"HTTP {resp.status_code}: {resp.text[:300]}")
                out[slot.name] = {"ok": True, "base_url": slot.base_url, "model": slot.model}
            except Exception as exc:  # noqa: BLE001
                out[slot.name] = {"ok": False, "error": str(exc)}
        return out

    def load(self) -> None:
        return

    def unload(self) -> None:
        return

    def maybe_unload(self, idle_seconds: int) -> None:
        return

    def status(self) -> dict[str, Any]:
        primary = self.primary.status()
        backup = self.backup.status()
        active = self.primary if self.primary.enabled and self.primary.configured else self.backup
        return {
            "enabled": self.enabled,
            "configured": self.configured,
            "provider": active.provider,
            "model": active.model,
            "base_url": active.base_url,
            "api_key_set": bool(active.api_key),
            "loaded": self.loaded,
            "last_used": max(self.primary.last_used, self.backup.last_used),
            "primary": primary,
            "backup": backup,
        }

    def verify(self, image: Image.Image, prompt: str, candidates: list[dict[str, Any]] | None = None) -> dict[str, Any]:
        slots = self._active_slots()
        if not slots:
            raise RuntimeError("VLM 已关闭或主/备均未配置（不启用本地回退）")
        data_url = self._data_url(image)
        content = prompt
        if candidates:
            content += "\n候选商品：\n" + json.dumps(candidates, ensure_ascii=False)
        errors: list[str] = []
        for slot in slots:
            try:
                text = self._call(slot, content, data_url)
                slot.last_used = time.monotonic()
                return {
                    "text": text,
                    "slot": slot.name,
                    "provider": slot.provider,
                    "model": slot.model,
                }
            except Exception as exc:  # noqa: BLE001
                errors.append(f"{slot.name}: {exc}")
                logger.warning("VLM %s failed: %s", slot.name, exc)
        raise RuntimeError("主/备 VLM 均不可用: " + "; ".join(errors))

    def _call(self, slot: VLMSlot, content: str, data_url: str) -> str:
        payload = {
            "model": slot.model,
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
        url = slot.base_url + "/chat/completions"
        with httpx.Client(timeout=settings.vlm_timeout_seconds) as client:
            resp = client.post(url, headers=self._headers(slot), json=payload)
        if resp.status_code < 200 or resp.status_code >= 300:
            raise RuntimeError(f"{slot.name} 返回 {resp.status_code}: {resp.text[:500]}")
        body = resp.json()
        choices = body.get("choices") or []
        if not choices:
            raise RuntimeError(f"{slot.name} 响应为空")
        message = choices[0].get("message") or {}
        return str(message.get("content") or "")

    def _headers(self, slot: VLMSlot) -> dict[str, str]:
        headers = {"Content-Type": "application/json"}
        if slot.api_key:
            headers["Authorization"] = f"Bearer {slot.api_key}"
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
        if "enabled" in data:
            self._global_enabled = bool(data["enabled"])
        if isinstance(data.get("primary"), dict):
            self._apply_slot(self.primary, data["primary"])
        if isinstance(data.get("backup"), dict):
            self._apply_slot(self.backup, data["backup"])
        # 兼容旧版单槽位配置
        if "base_url" in data or "model" in data or "api_key" in data:
            self._apply_slot(self.primary, data)

    def _persist(self) -> None:
        path = Path(settings.vlm_config_file)
        if not path.is_absolute():
            path = Path.cwd() / path
        path.parent.mkdir(parents=True, exist_ok=True)
        payload = {
            "enabled": self._global_enabled,
            "primary": {
                "enabled": self.primary.enabled,
                "provider": self.primary.provider,
                "base_url": self.primary.base_url,
                "model": self.primary.model,
                "api_key": self.primary.api_key,
            },
            "backup": {
                "enabled": self.backup.enabled,
                "provider": self.backup.provider,
                "base_url": self.backup.base_url,
                "model": self.backup.model,
                "api_key": self.backup.api_key,
            },
        }
        tmp = path.with_suffix(path.suffix + ".tmp")
        tmp.write_text(json.dumps(payload, ensure_ascii=False, indent=2), encoding="utf-8")
        try:
            os.chmod(tmp, 0o600)
        except OSError:
            pass
        os.replace(tmp, path)
