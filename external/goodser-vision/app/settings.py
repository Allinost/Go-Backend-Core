from __future__ import annotations

import os
from dataclasses import dataclass


def _env(name: str, default: str) -> str:
    return os.getenv(name, default).strip()


def _bool(name: str, default: bool) -> bool:
    raw = _env(name, "1" if default else "0").lower()
    return raw in {"1", "true", "yes", "on"}


@dataclass(frozen=True)
class Settings:
    app_name: str = _env("GOODSER_VISION_APP_NAME", "goodser-vision")
    version: str = _env("GOODSER_VISION_VERSION", "0.2.0")
    device: str = _env("DEVICE", "cpu").lower()
    embedder: str = _env("GOODSER_VISION_EMBEDDER", "fallback").lower()
    detector: str = _env("GOODSER_VISION_DETECTOR", "fallback").lower()
    embed_dim: int = int(_env("GOODSER_VISION_EMBED_DIM", "512"))
    max_image_side: int = int(_env("GOODSER_VISION_MAX_IMAGE_SIDE", "1920"))
    max_concurrency: int = int(_env("GOODSER_VISION_MAX_CONCURRENCY", "2"))
    model_dir: str = _env("GOODSER_VISION_MODEL_DIR", "models")
    gpu_memory_fraction: float = float(_env("GOODSER_VISION_GPU_MEMORY_FRACTION", "0.5"))
    gpu_headroom_mb: int = int(_env("GOODSER_VISION_GPU_HEADROOM_MB", "1024"))

    # 启动预热
    warmup: bool = _bool("GOODSER_VISION_WARMUP", True)
    # 不活跃卸载时间（秒），0 表示常驻不卸载
    idle_unload_seconds: int = int(_env("GOODSER_VISION_IDLE_UNLOAD_SECONDS", "0"))
    # 空闲扫描间隔
    idle_check_seconds: int = int(_env("GOODSER_VISION_IDLE_CHECK_SECONDS", "30"))

    # VLM
    vlm_enabled: bool = _bool("GOODSER_VISION_VLM_ENABLED", False)
    # auto | primary | backup
    vlm_mode: str = _env("GOODSER_VISION_VLM_MODE", "auto").lower()
    # 主失败后的冷却时间（秒），冷却期内直接走备
    vlm_cooldown_seconds: int = int(_env("GOODSER_VISION_VLM_COOLDOWN_SECONDS", "60"))
    # disabled | openai（OpenAI 兼容接口：vLLM / Ollama / 云厂商 / 订阅网关）
    vlm_provider: str = _env("GOODSER_VISION_VLM_PROVIDER", "disabled").lower()
    vlm_base_url: str = _env("GOODSER_VISION_VLM_BASE_URL", "")
    vlm_api_key: str = _env("GOODSER_VISION_VLM_API_KEY", "")
    vlm_model: str = _env("GOODSER_VISION_VLM_MODEL", "qwen2.5-vl-3b-instruct")
    vlm_timeout_seconds: int = int(_env("GOODSER_VISION_VLM_TIMEOUT_SECONDS", "60"))
    vlm_max_image_side: int = int(_env("GOODSER_VISION_VLM_MAX_IMAGE_SIDE", "1280"))
    vlm_max_tokens: int = int(_env("GOODSER_VISION_VLM_MAX_TOKENS", "512"))
    vlm_config_file: str = _env("GOODSER_VISION_VLM_CONFIG_FILE", "data/vlm.json")

    # VLM 主/备（兼容旧单槽位 VLM_*）
    vlm_primary_enabled: bool = _bool(
        "GOODSER_VISION_VLM_PRIMARY_ENABLED",
        _bool("GOODSER_VISION_VLM_ENABLED", False),
    )
    vlm_primary_provider: str = _env(
        "GOODSER_VISION_VLM_PRIMARY_PROVIDER",
        _env("GOODSER_VISION_VLM_PROVIDER", "disabled"),
    ).lower()
    vlm_primary_base_url: str = _env(
        "GOODSER_VISION_VLM_PRIMARY_BASE_URL",
        _env("GOODSER_VISION_VLM_BASE_URL", ""),
    )
    vlm_primary_api_key: str = _env(
        "GOODSER_VISION_VLM_PRIMARY_API_KEY",
        _env("GOODSER_VISION_VLM_API_KEY", ""),
    )
    vlm_primary_model: str = _env(
        "GOODSER_VISION_VLM_PRIMARY_MODEL",
        _env("GOODSER_VISION_VLM_MODEL", "qwen2.5-vl-3b-instruct"),
    )

    vlm_backup_enabled: bool = _bool("GOODSER_VISION_VLM_BACKUP_ENABLED", False)
    vlm_backup_provider: str = _env("GOODSER_VISION_VLM_BACKUP_PROVIDER", "disabled").lower()
    vlm_backup_base_url: str = _env("GOODSER_VISION_VLM_BACKUP_BASE_URL", "")
    vlm_backup_api_key: str = _env("GOODSER_VISION_VLM_BACKUP_API_KEY", "")
    vlm_backup_model: str = _env("GOODSER_VISION_VLM_BACKUP_MODEL", "")

    # OCR: gpu | cpu | auto（默认 GPU，初始化失败自动回退 CPU）
    ocr_enabled: bool = _bool("GOODSER_VISION_OCR_ENABLED", True)
    ocr_device: str = _env("GOODSER_VISION_OCR_DEVICE", "gpu").lower()
    ocr_lang: str = _env("GOODSER_VISION_OCR_LANG", "ch")


settings = Settings()
