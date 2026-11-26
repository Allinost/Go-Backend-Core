from __future__ import annotations

import os
from dataclasses import dataclass


def _env(name: str, default: str) -> str:
    return os.getenv(name, default).strip()


@dataclass(frozen=True)
class Settings:
    app_name: str = _env("GOODSER_VISION_APP_NAME", "goodser-vision")
    version: str = _env("GOODSER_VISION_VERSION", "0.1.0")
    device: str = _env("DEVICE", "cpu").lower()
    embedder: str = _env("GOODSER_VISION_EMBEDDER", "fallback").lower()
    detector: str = _env("GOODSER_VISION_DETECTOR", "fallback").lower()
    embed_dim: int = int(_env("GOODSER_VISION_EMBED_DIM", "512"))
    max_image_side: int = int(_env("GOODSER_VISION_MAX_IMAGE_SIDE", "1920"))
    max_concurrency: int = int(_env("GOODSER_VISION_MAX_CONCURRENCY", "2"))
    model_dir: str = _env("GOODSER_VISION_MODEL_DIR", "models")


settings = Settings()
