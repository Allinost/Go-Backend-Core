from __future__ import annotations

import asyncio
import base64
import io
import json
import time
from collections import Counter

from fastapi import FastAPI, File, Form, HTTPException, UploadFile
from fastapi.responses import PlainTextResponse
from PIL import Image, ImageOps
from pydantic import BaseModel

from .runtime import Runtime, vram_snapshot
from .schemas import (
    AnalyzeItem,
    AnalyzeResponse,
    Box,
    EmbedResponse,
    HealthResponse,
    ModelInfo,
    ModelsResponse,
    SegmentResponse,
)
from .settings import settings

app = FastAPI(title=settings.app_name, version=settings.version)

runtime = Runtime()
_semaphore = asyncio.Semaphore(max(1, settings.max_concurrency))
_started_at = time.time()
_metrics: Counter[str] = Counter()


class VLMToggle(BaseModel):
    enabled: bool


class VLMSlotUpdate(BaseModel):
    enabled: bool | None = None
    provider: str | None = None
    base_url: str | None = None
    model: str | None = None
    api_key: str | None = None


class VLMConfigUpdate(BaseModel):
    enabled: bool | None = None
    mode: str | None = None
    primary: VLMSlotUpdate | None = None
    backup: VLMSlotUpdate | None = None
    # 兼容旧单槽位字段
    provider: str | None = None
    base_url: str | None = None
    model: str | None = None
    api_key: str | None = None


class VLMSwitch(BaseModel):
    slot: str


@app.on_event("startup")
async def _startup() -> None:
    if settings.device == "cuda":
        try:
            import torch

            if torch.cuda.is_available():
                torch.cuda.set_per_process_memory_fraction(
                    max(0.1, min(0.95, settings.gpu_memory_fraction))
                )
                torch.cuda.empty_cache()
        except Exception:
            pass
    await runtime.start()


@app.on_event("shutdown")
async def _shutdown() -> None:
    await runtime.stop()


def _ensure_gpu_headroom() -> None:
    if settings.device != "cuda":
        return
    info = vram_snapshot()
    if not info.get("available"):
        return
    if info["free_bytes"] < settings.gpu_headroom_mb * 1024 * 1024:
        raise HTTPException(status_code=503, detail="GPU 显存余量不足，请稍后重试")


async def _load_image(upload: UploadFile) -> Image.Image:
    if upload.content_type and not upload.content_type.startswith("image/"):
        raise HTTPException(status_code=400, detail="仅支持图片文件")
    data = await upload.read()
    if not data:
        raise HTTPException(status_code=400, detail="图片为空")
    if len(data) > 20 * 1024 * 1024:
        raise HTTPException(status_code=400, detail="图片不能超过 20MB")
    try:
        image = Image.open(io.BytesIO(data))
        image = ImageOps.exif_transpose(image)
        image.load()
    except Exception as exc:  # noqa: BLE001
        raise HTTPException(status_code=400, detail=f"图片解析失败: {exc}") from exc
    if max(image.size) > settings.max_image_side:
        image.thumbnail((settings.max_image_side, settings.max_image_side), Image.Resampling.LANCZOS)
    return image.convert("RGB")


def _crop(image: Image.Image, box: Box) -> Image.Image:
    left = max(0, box.x)
    top = max(0, box.y)
    right = min(image.width, box.x + box.w)
    bottom = min(image.height, box.y + box.h)
    return image.crop((left, top, max(left + 1, right), max(top + 1, bottom)))


def _data_url(image: Image.Image) -> str:
    buf = io.BytesIO()
    image.save(buf, format="JPEG", quality=88)
    encoded = base64.b64encode(buf.getvalue()).decode("ascii")
    return f"data:image/jpeg;base64,{encoded}"


@app.get("/healthz", response_model=HealthResponse)
async def healthz() -> HealthResponse:
    status = runtime.status()
    return HealthResponse(
        status="ok",
        embedder=status["models"]["embedder"]["name"],
        detector=status["models"]["detector"]["name"],
        device=settings.device,
    )


@app.get("/v1/models", response_model=ModelsResponse)
async def models() -> ModelsResponse:
    status = runtime.status()
    embedder = status["models"]["embedder"]
    detector = status["models"]["detector"]
    ocr = status["models"]["ocr"]
    vlm = status["models"]["vlm"]
    return ModelsResponse(
        embedder=ModelInfo(
            name=embedder["name"],
            version=f'{settings.version}:{"loaded" if embedder["loaded"] else "lazy"}',
            device=settings.device,
            dim=embedder["dim"],
        ),
        detector=ModelInfo(
            name=detector["name"],
            version=f'{settings.version}:{"loaded" if detector["loaded"] else "fallback"}',
            device=settings.device,
            dim=0,
        ),
        ocr=ModelInfo(
            name=ocr["name"],
            version=f'{settings.version}:{"loaded" if ocr["loaded"] else "disabled"}',
            device=ocr["device"],
            dim=0,
        ),
        vlm=ModelInfo(
            name=vlm["model"],
            version=f'{vlm["provider"]}:{"enabled" if vlm["enabled"] else "disabled"}',
            device="external",
            dim=0,
        ),
    )


@app.get("/v1/system/status")
async def system_status() -> dict:
    return runtime.status()


@app.post("/v1/system/vlm")
async def set_vlm(payload: VLMToggle) -> dict:
    runtime.vlm.set_enabled(payload.enabled)
    return runtime.vlm.status()


@app.get("/v1/system/vlm")
async def get_vlm() -> dict:
    return runtime.vlm.status()


@app.put("/v1/system/vlm")
async def update_vlm(payload: VLMConfigUpdate) -> dict:
    primary = payload.primary.model_dump(exclude_none=True) if payload.primary else None
    backup = payload.backup.model_dump(exclude_none=True) if payload.backup else None
    return runtime.vlm.update(
        enabled=payload.enabled,
        mode=payload.mode,
        primary=primary,
        backup=backup,
        provider=payload.provider,
        base_url=payload.base_url,
        model=payload.model,
        api_key=payload.api_key,
    )


@app.post("/v1/system/vlm/switch")
async def switch_vlm(payload: VLMSwitch) -> dict:
    try:
        return runtime.vlm.switch(payload.slot)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc


@app.post("/v1/system/vlm/test")
async def test_vlm(slot: str | None = None) -> dict:
    try:
        return await asyncio.to_thread(runtime.vlm.test, slot)
    except Exception as exc:  # noqa: BLE001
        raise HTTPException(status_code=503, detail=f"New API 不可用: {exc}") from exc


@app.post("/v1/embed", response_model=EmbedResponse)
async def embed(image: UploadFile = File(...)) -> EmbedResponse:
    _ensure_gpu_headroom()
    async with _semaphore:
        img = await _load_image(image)
        vector = await asyncio.to_thread(runtime.embedder.embed, img)
    _metrics["embed"] += 1
    return EmbedResponse(model=runtime.embedder.name, dim=len(vector), vector=vector)


@app.post("/v1/embed/batch", response_model=list[EmbedResponse])
async def embed_batch(images: list[UploadFile] = File(...)) -> list[EmbedResponse]:
    _ensure_gpu_headroom()
    if len(images) > 50:
        raise HTTPException(status_code=400, detail="单次最多 50 张")
    out: list[EmbedResponse] = []
    async with _semaphore:
        for upload in images:
            img = await _load_image(upload)
            vector = await asyncio.to_thread(runtime.embedder.embed, img)
            out.append(EmbedResponse(model=runtime.embedder.name, dim=len(vector), vector=vector))
    _metrics["embed_batch"] += len(out)
    return out


@app.post("/v1/analyze", response_model=AnalyzeResponse)
async def analyze(
    image: UploadFile = File(...),
    mode: str = Form("multi"),
    top_k: int = Form(5),
) -> AnalyzeResponse:
    _ensure_gpu_headroom()
    started = time.perf_counter()
    async with _semaphore:
        img = await _load_image(image)
        boxes = await asyncio.to_thread(runtime.detector.detect, img)
        items: list[AnalyzeItem] = []
        vectors: list[list[float]] = []
        for box in boxes:
            crop = _crop(img, box)
            vector = await asyncio.to_thread(runtime.embedder.embed, crop)
            vectors.append(vector)
            items.append(AnalyzeItem(box=box, vector=vector, candidates=[]))
        groups = []
        if mode == "multi":
            from .services.matcher import group_by_similarity

            groups = group_by_similarity(vectors, list(range(len(vectors))))
    _metrics["analyze"] += 1
    return AnalyzeResponse(
        model={
            "detector": runtime.detector.name,
            "embedder": runtime.embedder.name,
            "dim": runtime.embedder.dim,
            "mode": mode,
            "top_k": top_k,
        },
        elapsed_ms=int((time.perf_counter() - started) * 1000),
        items=items,
        groups=groups,
    )


@app.post("/v1/segment", response_model=SegmentResponse)
async def segment(image: UploadFile = File(...)) -> SegmentResponse:
    _ensure_gpu_headroom()
    started = time.perf_counter()
    async with _semaphore:
        img = await _load_image(image)
        boxes = await asyncio.to_thread(runtime.detector.detect, img)
        crops = [_data_url(_crop(img, box)) for box in boxes]
    _metrics["segment"] += 1
    return SegmentResponse(
        model={
            "detector": runtime.detector.name,
            "version": settings.version,
            "loaded": runtime.detector.loaded,
        },
        elapsed_ms=int((time.perf_counter() - started) * 1000),
        boxes=boxes,
        crops=crops,
    )


@app.post("/v1/ocr")
async def ocr(image: UploadFile = File(...)) -> dict:
    img = await _load_image(image)
    items = await asyncio.to_thread(runtime.ocr.recognize, img)
    return {"items": items}


@app.post("/v1/verify")
async def verify(
    image: UploadFile = File(...),
    prompt: str = Form("请判断图中商品是否与候选商品一致，并说明理由。"),
    candidates: str = Form("[]"),
) -> dict:
    try:
        parsed = json.loads(candidates) if candidates else []
    except json.JSONDecodeError:
        parsed = []
    img = await _load_image(image)
    try:
        result = await asyncio.to_thread(runtime.vlm.verify, img, prompt, parsed)
    except Exception as exc:  # noqa: BLE001
        raise HTTPException(status_code=503, detail=f"VLM 不可用: {exc}") from exc
    _metrics["verify"] += 1
    return result


@app.get("/metrics", response_class=PlainTextResponse)
async def metrics() -> str:
    lines = [
        "# HELP goodser_vision_requests_total Total requests by endpoint",
        "# TYPE goodser_vision_requests_total counter",
    ]
    for key, value in sorted(_metrics.items()):
        lines.append(f'goodser_vision_requests_total{{endpoint="{key}"}} {value}')
    lines.extend(runtime.metrics_lines())
    return "\n".join(lines) + "\n"
