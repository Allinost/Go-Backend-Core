from __future__ import annotations

import hashlib
import io
import logging
import threading
import time
from dataclasses import dataclass

import numpy as np
from PIL import Image, ImageOps

from ..settings import settings

logger = logging.getLogger("goodser-vision.embedder")


@dataclass
class EmbedderStatus:
    name: str
    dim: int
    loaded: bool
    last_used: float
    device: str


class Embedder:
    """Chinese-CLIP 常驻单例；fallback 模式不占显存。"""

    def __init__(self) -> None:
        self.name = f"{settings.embedder}-v1"
        self.dim = int(settings.embed_dim)
        self._model = None
        self._processor = None
        self._lock = threading.RLock()
        self._last_used = 0.0
        self._in_use = 0
        self._failed = False

    @property
    def loaded(self) -> bool:
        return self._model is not None

    def load(self) -> None:
        if settings.embedder != "clip" or self._model is not None or self._failed:
            return
        with self._lock:
            if self._model is not None:
                return
            try:
                import torch
                from transformers import ChineseCLIPModel, ChineseCLIPProcessor

                model_name = "OFA-Sys/chinese-clip-vit-base-patch16"
                logger.info("loading Chinese-CLIP: %s", model_name)
                self._processor = ChineseCLIPProcessor.from_pretrained(
                    model_name, cache_dir=settings.model_dir
                )
                model = ChineseCLIPModel.from_pretrained(
                    model_name, cache_dir=settings.model_dir
                )
                model.eval()
                if settings.device == "cuda" and torch.cuda.is_available():
                    model.to("cuda")
                self._model = model
                logger.info("Chinese-CLIP loaded")
            except Exception as exc:  # noqa: BLE001
                logger.warning("Chinese-CLIP load failed, fallback in use: %s", exc)
                self._model = None
                self._processor = None
                self._failed = True

    def unload(self) -> None:
        with self._lock:
            if self._model is None:
                return
            self._model = None
            self._processor = None
            try:
                import gc

                import torch

                gc.collect()
                if torch.cuda.is_available():
                    torch.cuda.empty_cache()
            except Exception:
                pass
            logger.info("Chinese-CLIP unloaded")

    def maybe_unload(self, idle_seconds: int) -> None:
        if idle_seconds <= 0 or not self.loaded or self._in_use > 0:
            return
        if time.monotonic() - self._last_used > idle_seconds:
            self.unload()

    def embed(self, image: Image.Image) -> list[float]:
        self._in_use += 1
        try:
            self.load()
            self._last_used = time.monotonic()
            if self._model is not None and self._processor is not None:
                return self._embed_clip(image)
            return self._embed_fallback(image)
        finally:
            self._in_use = max(0, self._in_use - 1)

    def _embed_fallback(self, image: Image.Image) -> list[float]:
        img = ImageOps.exif_transpose(image).convert("RGB").resize((16, 16), Image.Resampling.BILINEAR)
        arr = np.asarray(img, dtype=np.float32).reshape(-1) / 255.0
        dim = self.dim
        if arr.size >= dim:
            vec = arr[:dim]
        else:
            vec = np.pad(arr, (0, dim - arr.size))
        norm = float(np.linalg.norm(vec))
        if norm > 0:
            vec = vec / norm
        return [round(float(v), 6) for v in vec]

    def _embed_clip(self, image: Image.Image) -> list[float]:
        import torch

        inputs = self._processor(images=image.convert("RGB"), return_tensors="pt")
        if settings.device == "cuda" and torch.cuda.is_available():
            inputs = {key: value.to("cuda") for key, value in inputs.items()}
        with torch.inference_mode():
            features = self._model.get_image_features(**inputs)
        vec = features[0].detach().float().cpu().numpy()
        norm = float(np.linalg.norm(vec))
        if norm > 0:
            vec = vec / norm
        return [round(float(v), 6) for v in vec.tolist()]

    def status(self) -> EmbedderStatus:
        return EmbedderStatus(
            name=self.name,
            dim=self.dim,
            loaded=self.loaded,
            last_used=self._last_used,
            device=settings.device,
        )


def image_digest(image: Image.Image) -> str:
    buf = io.BytesIO()
    image.convert("RGB").save(buf, format="JPEG", quality=85)
    return hashlib.sha256(buf.getvalue()).hexdigest()[:16]
