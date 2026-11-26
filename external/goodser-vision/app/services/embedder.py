from __future__ import annotations

import hashlib
import io
import math
from dataclasses import dataclass

import numpy as np
from PIL import Image, ImageOps

from ..settings import settings


@dataclass
class Embedder:
    name: str
    dim: int

    def embed(self, image: Image.Image) -> list[float]:
        if settings.embedder == "clip":
            try:
                return self._embed_clip(image)
            except Exception:
                # 模型未安装/加载失败时回退，保证接口可用
                pass
        return self._embed_fallback(image)

    def _embed_fallback(self, image: Image.Image) -> list[float]:
        # 轻量 fallback：16x16 RGB 网格，截断/补齐到 512 维并 L2 归一化
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
        # 可选：安装 torch/transformers 后启用
        import torch
        from transformers import ChineseCLIPModel, ChineseCLIPProcessor

        model_name = "OFA-Sys/chinese-clip-vit-base-patch16"
        processor = ChineseCLIPProcessor.from_pretrained(model_name, cache_dir=settings.model_dir)
        model = ChineseCLIPModel.from_pretrained(model_name, cache_dir=settings.model_dir)
        model.eval()
        inputs = processor(images=image.convert("RGB"), return_tensors="pt")
        with torch.no_grad():
            features = model.get_image_features(**inputs)
        vec = features[0].cpu().numpy().astype(np.float32)
        norm = float(np.linalg.norm(vec))
        if norm > 0:
            vec = vec / norm
        return [round(float(v), 6) for v in vec.tolist()]


def get_embedder() -> Embedder:
    return Embedder(name=f"{settings.embedder}-v1", dim=int(settings.embed_dim))


def image_digest(image: Image.Image) -> str:
    buf = io.BytesIO()
    image.convert("RGB").save(buf, format="JPEG", quality=85)
    return hashlib.sha256(buf.getvalue()).hexdigest()[:16]
