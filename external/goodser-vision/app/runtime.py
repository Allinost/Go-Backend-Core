from __future__ import annotations

import asyncio
import logging
import time
from typing import Any

from PIL import Image

from .services.detector import Detector
from .services.embedder import Embedder
from .services.ocr import OCR
from .services.vlm import VLM
from .settings import settings

logger = logging.getLogger("goodser-vision.runtime")


class Runtime:
    def __init__(self) -> None:
        self.embedder = Embedder()
        self.detector = Detector()
        self.ocr = OCR()
        self.vlm = VLM()
        self._task: asyncio.Task | None = None
        self._started_at = time.time()

    async def start(self) -> None:
        if settings.warmup:
            await asyncio.to_thread(self._warmup)
        self._task = asyncio.create_task(self._idle_loop())

    async def stop(self) -> None:
        if self._task:
            self._task.cancel()
            try:
                await self._task
            except asyncio.CancelledError:
                pass
            self._task = None

    def _warmup(self) -> None:
        image = Image.new("RGB", (64, 64), "white")
        try:
            logger.info("warmup: embedder")
            self.embedder.embed(image)
        except Exception as exc:  # noqa: BLE001
            logger.warning("warmup embedder failed: %s", exc)
        try:
            logger.info("warmup: detector")
            self.detector.detect(image)
        except Exception as exc:  # noqa: BLE001
            logger.warning("warmup detector failed: %s", exc)
        try:
            logger.info("warmup: ocr")
            self.ocr.recognize(image)
        except Exception as exc:  # noqa: BLE001
            logger.warning("warmup ocr failed: %s", exc)
        logger.info("warmup done")

    async def _idle_loop(self) -> None:
        interval = max(5, settings.idle_check_seconds)
        while True:
            await asyncio.sleep(interval)
            idle = settings.idle_unload_seconds
            if idle <= 0:
                continue
            for item in (self.vlm, self.ocr, self.detector, self.embedder):
                try:
                    item.maybe_unload(idle)
                except Exception as exc:  # noqa: BLE001
                    logger.warning("idle unload failed: %s", exc)

    def status(self) -> dict[str, Any]:
        return {
            "version": settings.version,
            "device": settings.device,
            "idle_unload_seconds": settings.idle_unload_seconds,
            "warmup": settings.warmup,
            "uptime_seconds": int(time.time() - self._started_at),
            "models": {
                "embedder": self.embedder.status().__dict__,
                "detector": self.detector.status().__dict__,
                "ocr": self.ocr.status().__dict__,
                "vlm": self.vlm.status(),
            },
            "vram": vram_snapshot(),
        }

    def metrics_lines(self) -> list[str]:
        lines = [
            "# HELP goodser_vision_model_loaded Model loaded state",
            "# TYPE goodser_vision_model_loaded gauge",
        ]
        for name, model in (
            ("embedder", self.embedder),
            ("detector", self.detector),
            ("ocr", self.ocr),
            ("vlm", self.vlm),
        ):
            lines.append(
                f'goodser_vision_model_loaded{{model="{name}"}} {1 if getattr(model, "loaded", False) else 0}'
            )
        vram = vram_snapshot()
        if vram.get("available"):
            lines.extend(
                [
                    "# HELP goodser_vision_gpu_memory_bytes GPU memory by state",
                    "# TYPE goodser_vision_gpu_memory_bytes gauge",
                    f'goodser_vision_gpu_memory_bytes{{state="used"}} {vram["used_bytes"]}',
                    f'goodser_vision_gpu_memory_bytes{{state="free"}} {vram["free_bytes"]}',
                    f'goodser_vision_gpu_memory_bytes{{state="total"}} {vram["total_bytes"]}',
                    f'goodser_vision_gpu_memory_bytes{{state="torch_allocated"}} {vram["allocated_bytes"]}',
                    f'goodser_vision_gpu_memory_bytes{{state="torch_reserved"}} {vram["reserved_bytes"]}',
                ]
            )
        lines.append(f"goodser_vision_uptime_seconds {int(time.time() - self._started_at)}")
        return lines


def vram_snapshot() -> dict[str, Any]:
    try:
        import torch

        if not torch.cuda.is_available():
            return {"available": False}
        free, total = torch.cuda.mem_get_info()
        return {
            "available": True,
            "free_bytes": int(free),
            "total_bytes": int(total),
            "used_bytes": int(total - free),
            "allocated_bytes": int(torch.cuda.memory_allocated()),
            "reserved_bytes": int(torch.cuda.memory_reserved()),
            "device_name": torch.cuda.get_device_name(0),
        }
    except Exception as exc:  # noqa: BLE001
        return {"available": False, "error": str(exc)}
