from __future__ import annotations

import logging
import threading
import time
from dataclasses import dataclass
from typing import Any

from PIL import Image

from ..settings import settings

logger = logging.getLogger("goodser-vision.ocr")


def _to_plain(value: Any) -> Any:
    """把 numpy 数组/元组转成可 JSON 序列化的 list。"""
    tolist = getattr(value, "tolist", None)
    if callable(tolist):
        return tolist()
    if isinstance(value, (list, tuple)):
        return [_to_plain(item) for item in value]
    return value


@dataclass
class OCRStatus:
    name: str
    loaded: bool
    last_used: float
    device: str


class OCR:
    """PaddleOCR 常驻单例；设备可配置 cpu/gpu/auto，未安装时返回空结果。"""

    def __init__(self) -> None:
        self.name = "paddleocr-v1"
        self.enabled = bool(settings.ocr_enabled)
        self.device = settings.ocr_device
        self.lang = settings.ocr_lang
        self._device = "cpu"
        self._engine = None
        self._lock = threading.RLock()
        self._last_used = 0.0
        self._in_use = 0
        self._failed = False

    @property
    def loaded(self) -> bool:
        return self._engine is not None

    def load(self) -> None:
        if not self.enabled or self._engine is not None or self._failed:
            return
        with self._lock:
            if self._engine is not None:
                return
            use_gpu = self.device == "gpu" or (self.device == "auto" and settings.device == "cuda")
            try:
                self._engine = self._create_engine(use_gpu)
                self._device = "gpu" if use_gpu else "cpu"
                logger.info("PaddleOCR loaded on %s", self._device)
            except Exception as exc:  # noqa: BLE001
                if use_gpu:
                    logger.warning("PaddleOCR GPU load failed, fallback to CPU: %s", exc)
                    try:
                        self._engine = self._create_engine(False)
                        self._device = "cpu"
                        logger.info("PaddleOCR loaded on cpu")
                        return
                    except Exception as cpu_exc:  # noqa: BLE001
                        logger.warning("PaddleOCR CPU load failed: %s", cpu_exc)
                else:
                    logger.warning("PaddleOCR load failed, OCR disabled: %s", exc)
                self._engine = None
                self._failed = True

    def _create_engine(self, use_gpu: bool):
        from paddleocr import PaddleOCR

        device = "gpu" if use_gpu else "cpu"
        # 兼容 PaddleOCR 2.x（use_gpu/show_log）与 3.x（device/use_textline_orientation）
        candidates: list[dict[str, Any]] = [
            {"lang": self.lang, "use_angle_cls": True, "show_log": False, "use_gpu": use_gpu},
            {"lang": self.lang, "use_angle_cls": True, "device": device},
            {"lang": self.lang, "use_textline_orientation": True, "device": device},
            {"lang": self.lang, "device": device},
        ]
        last_exc: Exception | None = None
        for kwargs in candidates:
            try:
                return PaddleOCR(**kwargs)
            except (TypeError, ValueError) as exc:
                # 参数签名不匹配，换下一种写法
                last_exc = exc
                continue
        if last_exc is not None:
            raise last_exc
        raise RuntimeError("PaddleOCR 初始化失败")

    def unload(self) -> None:
        with self._lock:
            if self._engine is None:
                return
            self._engine = None
            logger.info("PaddleOCR unloaded")

    def maybe_unload(self, idle_seconds: int) -> None:
        if idle_seconds <= 0 or not self.loaded or self._in_use > 0:
            return
        if time.monotonic() - self._last_used > idle_seconds:
            self.unload()

    def recognize(self, image: Image.Image) -> list[dict]:
        if not self.enabled:
            return []
        self._in_use += 1
        try:
            self.load()
            self._last_used = time.monotonic()
            if self._engine is None:
                return []
            result = self._engine.ocr(image.convert("RGB"))
            return self._parse_result(result)
        finally:
            self._in_use = max(0, self._in_use - 1)

    @staticmethod
    def _parse_result(result: Any) -> list[dict]:
        items: list[dict] = []
        for page in result or []:
            # PaddleOCR 3.x：page 为 dict-like，字段 rec_texts / rec_scores / rec_polys
            if isinstance(page, dict):
                texts = page.get("rec_texts") or []
                scores = page.get("rec_scores") or []
                polys = page.get("rec_polys") or page.get("dt_polys") or []
                for index, text in enumerate(texts):
                    if not text:
                        continue
                    items.append(
                        {
                            "box": _to_plain(polys[index]) if index < len(polys) else [],
                            "text": str(text),
                            "score": float(scores[index]) if index < len(scores) else 0.0,
                        }
                    )
                continue
            # PaddleOCR 2.x：page 为 [[box, (text, score)], ...]
            for line in page or []:
                if not line or len(line) < 2:
                    continue
                box, payload = line[0], line[1]
                if isinstance(payload, (list, tuple)) and len(payload) >= 2:
                    text, score = payload[0], payload[1]
                else:
                    text, score = payload, 0.0
                items.append(
                    {
                        "box": _to_plain(box),
                        "text": str(text),
                        "score": float(score),
                    }
                )
        return items

    def status(self) -> OCRStatus:
        return OCRStatus(
            name=self.name,
            loaded=self.loaded,
            last_used=self._last_used,
            device=self._device if self.loaded else (self.device or "cpu"),
        )
