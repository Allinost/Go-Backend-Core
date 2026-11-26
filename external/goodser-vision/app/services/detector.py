from __future__ import annotations

from dataclasses import dataclass

from PIL import Image

from ..schemas import Box
from ..settings import settings


@dataclass
class Detector:
    name: str

    def detect(self, image: Image.Image) -> list[Box]:
        if settings.detector == "yolo":
            try:
                return self._detect_yolo(image)
            except Exception:
                pass
        return self._detect_fallback(image)

    def _detect_fallback(self, image: Image.Image) -> list[Box]:
        # fallback：整图作为单框，保证链路可用；正式检测由 YOLO/Grounding DINO 替换
        return [Box(x=0, y=0, w=image.width, h=image.height, score=1.0)]

    def _detect_yolo(self, image: Image.Image) -> list[Box]:
        # 可选：安装 ultralytics 后启用
        from ultralytics import YOLO

        model = YOLO(f"{settings.model_dir}/yolo11s-goods.pt")
        results = model.predict(image.convert("RGB"), conf=0.25, iou=0.5, verbose=False)
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


def get_detector() -> Detector:
    return Detector(name=f"{settings.detector}-v1")
