from __future__ import annotations

from typing import Any

from pydantic import BaseModel, Field


class ModelInfo(BaseModel):
    name: str
    version: str
    device: str
    dim: int


class ModelsResponse(BaseModel):
    embedder: ModelInfo
    detector: ModelInfo
    ocr: ModelInfo | None = None
    vlm: ModelInfo | None = None


class EmbedResponse(BaseModel):
    model: str
    dim: int
    vector: list[float]


class Box(BaseModel):
    x: int
    y: int
    w: int
    h: int
    score: float = 1.0


class AnalyzeItem(BaseModel):
    box: Box
    crop_url: str | None = None
    vector: list[float] | None = None
    candidates: list[dict[str, Any]] = Field(default_factory=list)


class AnalyzeResponse(BaseModel):
    model: dict[str, Any]
    elapsed_ms: int
    items: list[AnalyzeItem]
    groups: list[dict[str, Any]] = Field(default_factory=list)


class SegmentResponse(BaseModel):
    model: dict[str, Any]
    elapsed_ms: int
    boxes: list[Box]
    crops: list[str] = Field(default_factory=list)


class HealthResponse(BaseModel):
    status: str
    embedder: str
    detector: str
    device: str
