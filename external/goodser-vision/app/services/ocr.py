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
    """PaddleOCR 常驻单例；未安装时返回空结果。"""

    def __init__(self) -> None:
        self.name = "paddleocr-v1"
        self._engine = None
        self._lock = threading.RLock()
        self._last_used = 0.0
        self._in_use = 0
        self._failed = False

    @property
    def loaded(self) -> bool:
        return self._engine is not None

    def load(self) -> None:
        if self._engine is not None or self._failed:
            return
        with self._lock:
            if self._engine is not None:
                return
            try:
                from paddleocr import PaddleOCR

                logger.info("loading PaddleOCR")
                self._engine = PaddleOCR(
                    use_angle_cls=True,
                    lang="ch",
                    show_log=False,
                )
                logger.info("PaddleOCR loaded")
            except Exception as exc:  # noqa: BLE001
                logger.warning("PaddleOCR load failed, OCR disabled: %s", exc)
                self._engine = None
                self._failed = True

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
            device="gpu" if self.loaded else "unknown",
        )
