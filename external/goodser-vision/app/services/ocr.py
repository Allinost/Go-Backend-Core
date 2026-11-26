from __future__ import annotations

import logging
import threading
import time
from dataclasses import dataclass

from PIL import Image

logger = logging.getLogger("goodser-vision.ocr")


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

        try:
            return PaddleOCR(
                use_angle_cls=True,
                lang=self.lang,
                show_log=False,
                use_gpu=use_gpu,
            )
        except TypeError:
            # PaddleOCR 3.x 部分版本移除了 use_gpu/show_log
            return PaddleOCR(
                use_angle_cls=True,
                lang=self.lang,
                device="gpu" if use_gpu else "cpu",
            )

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
            result = self._engine.ocr(image.convert("RGB"), cls=True)
            items: list[dict] = []
            for page in result or []:
                for line in page or []:
                    if not line or len(line) < 2:
                        continue
                    box, (text, score) = line[0], line[1]
                    items.append(
                        {
                            "box": box,
                            "text": text,
                            "score": float(score),
                        }
                    )
            return items
        finally:
            self._in_use = max(0, self._in_use - 1)

    def status(self) -> OCRStatus:
        return OCRStatus(
            name=self.name,
            loaded=self.loaded,
            last_used=self._last_used,
            device=self._device if self.loaded else (self.device or "cpu"),
        )
