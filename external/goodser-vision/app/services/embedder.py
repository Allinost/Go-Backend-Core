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


def _huggingface_reachable(timeout: float = 3.0) -> bool:
    """快速判断 HuggingFace（或 HF_ENDPOINT 镜像）是否可达。

    无外网时 transformers/huggingface_hub 会按文件逐个重试（每次 5 轮、指数退避），
    导致 FastAPI 启动被阻塞数分钟、端口不可用。这里先做一次 3 秒 TCP 探测，
    不可达则立即回退 fallback embedder。
    """
    import os
    import socket
    from urllib.parse import urlparse

    endpoint = os.getenv("HF_ENDPOINT", "https://huggingface.co")
    host = urlparse(endpoint).hostname or "huggingface.co"
    try:
        with socket.create_connection((host, 443), timeout=timeout):
            return True
    except OSError:
        return False


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
                try:
                    # 优先离线加载已缓存的模型，避免无外网时长时间重试
                    self._processor = ChineseCLIPProcessor.from_pretrained(
                        model_name, cache_dir=settings.model_dir, local_files_only=True
                    )
                    model = ChineseCLIPModel.from_pretrained(
                        model_name, cache_dir=settings.model_dir, local_files_only=True
                    )
                except Exception:
                    if not _huggingface_reachable():
                        raise RuntimeError("HuggingFace 不可达，且本地无 Chinese-CLIP 缓存")
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
            outputs = self._model.get_image_features(**inputs)
        features = self._extract_features(outputs)
        # 不同 transformers 版本返回形状可能是 (1, D) / (1, 1, D)，
        # 直接 tolist() 会得到嵌套 list，float() 会报 TypeError，这里统一拉平。
        vec = np.asarray(features.detach().float().cpu().numpy(), dtype=np.float32).reshape(-1)
        dim = int(self.dim)
        if vec.size != dim:
            logger.warning(
                "CLIP 输出维度 %d 与配置 %d 不一致，按配置截断/补零", vec.size, dim
            )
            if vec.size > dim:
                vec = vec[:dim]
            else:
                vec = np.pad(vec, (0, dim - vec.size))
        norm = float(np.linalg.norm(vec))
        if norm > 0:
            vec = vec / norm
        return [round(float(v), 6) for v in vec.tolist()]

    @staticmethod
    def _extract_features(outputs):
        """兼容 transformers 各版本：Tensor / 元组 / BaseModelOutput。"""
        import torch

        if isinstance(outputs, torch.Tensor):
            return outputs
        if isinstance(outputs, (list, tuple)):
            for item in outputs:
                if isinstance(item, torch.Tensor):
                    return item
        for attr in ("image_embeds", "pooler_output", "last_hidden_state"):
            value = getattr(outputs, attr, None)
            if isinstance(value, torch.Tensor):
                return value
        raise RuntimeError(f"无法从 CLIP 输出中提取特征: {type(outputs)!r}")

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
