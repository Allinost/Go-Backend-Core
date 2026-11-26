from __future__ import annotations

import logging
import os
import threading
import time
from dataclasses import dataclass

from PIL import Image

from ..schemas import Box
from ..settings import settings

logger = logging.getLogger("goodser-vision.detector")


@dataclass
class DetectorStatus:
    name: str
    loaded: bool
    last_used: float
    device: str


class Detector:
    """YOLO 常驻单例；权重缺失时回退整图单框。"""

    def __init__(self) -> None:
        self.name = f"{settings.detector}-v1"
        self._model = None
        self._lock = threading.RLock()
        self._last_used = 0.0
        self._in_use = 0
        self._failed = False

    @property
    def loaded(self) -> bool:
        return self._model is not None

    def load(self) -> None:
        if settings.detector != "yolo" or self._model is not None or self._failed:
            return
        with self._lock:
            if self._model is not None:
                return
            weight = os.path.join(settings.model_dir, "yolo11s-goods.pt")
            if not os.path.exists(weight):
                logger.warning("YOLO weight not found: %s, fallback in use", weight)
                self._failed = True
                return
            try:
                from ultralytics import YOLO

                logger.info("loading YOLO: %s", weight)
                self._model = YOLO(weight)
                logger.info("YOLO loaded")
            except Exception as exc:  # noqa: BLE001
                logger.warning("YOLO load failed, fallback in use: %s", exc)
                self._model = None
                self._failed = True

    def unload(self) -> None:
        with self._lock:
            if self._model is None:
                return
            self._model = None
            try:
                import gc

                import torch

                gc.collect()
                if torch.cuda.is_available():
                    torch.cuda.empty_cache()
            except Exception:
                pass
            logger.info("YOLO unloaded")

    def maybe_unload(self, idle_seconds: int) -> None:
        if idle_seconds <= 0 or not self.loaded or self._in_use > 0:
            return
        if time.monotonic() - self._last_used > idle_seconds:
            self.unload()

    def detect(self, image: Image.Image) -> list[Box]:
        self._in_use += 1
        try:
            self.load()
            self._last_used = time.monotonic()
            if self._model is not None:
                return self._detect_yolo(image)
            return self._detect_fallback(image)
        finally:
            self._in_use = max(0, self._in_use - 1)

    def _detect_fallback(self, image: Image.Image) -> list[Box]:
        return [Box(x=0, y=0, w=image.width, h=image.height, score=1.0)]

    def _detect_yolo(self, image: Image.Image) -> list[Box]:
        results = self._model.predict(image.convert("RGB"), conf=0.25, iou=0.5, verbose=False)
        boxes: list[Box] = []
        for result in results:
            for b in result.boxes:
                x1, y1, x2, y2 = [float(v) for v in b.xyxy[0].tolist()]
                score = float(b.conf[0]) if b.conf is not None and len(b.conf) else 1.0
                boxes.append(
                    Box(
                        x=max(0, int(x1)),
                        y=max(0, int(y1)),
                        w=max(1, int(x2 - x1)),
                        h=max(1, int(y2 - y1)),
                        score=round(score, 4),
                    )
                )
        return boxes or self._detect_fallback(image)

    def status(self) -> DetectorStatus:
        return DetectorStatus(
            name=self.name,
            loaded=self.loaded,
            last_used=self._last_used,
            device=settings.device,
        )


def get_detector() -> Detector:
    return Detector()
