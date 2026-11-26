from __future__ import annotations

import asyncio
import base64
import io
import time
from collections import Counter

from fastapi import FastAPI, File, Form, HTTPException, UploadFile
from fastapi.responses import PlainTextResponse
from PIL import Image, ImageOps

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
from .services.detector import get_detector
from .services.embedder import get_embedder
from .services.matcher import group_by_similarity
from .services.ocr import ocr as run_ocr
from .settings import settings

app = FastAPI(title=settings.app_name, version=settings.version)

_embedder = get_embedder()
_detector = get_detector()
_semaphore = asyncio.Semaphore(max(1, settings.max_concurrency))
_started_at = time.time()
_metrics: Counter[str] = Counter()


def _now_ms() -> int:
    return int((time.time() - _started_at) * 1000)


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
    return HealthResponse(
        status="ok",
        embedder=_embedder.name,
        detector=_detector.name,
        device=settings.device,
    )


@app.get("/v1/models", response_model=ModelsResponse)
async def models() -> ModelsResponse:
    return ModelsResponse(
        embedder=ModelInfo(name=_embedder.name, version=settings.version, device=settings.device, dim=_embedder.dim),
        detector=ModelInfo(name=_detector.name, version=settings.version, device=settings.device, dim=0),
        ocr=ModelInfo(name="placeholder", version=settings.version, device="cpu", dim=0),
    )


@app.post("/v1/embed", response_model=EmbedResponse)
async def embed(image: UploadFile = File(...)) -> EmbedResponse:
    async with _semaphore:
        img = await _load_image(image)
        vector = await asyncio.to_thread(_embedder.embed, img)
    _metrics["embed"] += 1
    return EmbedResponse(model=_embedder.name, dim=len(vector), vector=vector)


@app.post("/v1/embed/batch", response_model=list[EmbedResponse])
async def embed_batch(images: list[UploadFile] = File(...)) -> list[EmbedResponse]:
    if len(images) > 50:
        raise HTTPException(status_code=400, detail="单次最多 50 张")
    out: list[EmbedResponse] = []
    async with _semaphore:
        for upload in images:
            img = await _load_image(upload)
            vector = await asyncio.to_thread(_embedder.embed, img)
            out.append(EmbedResponse(model=_embedder.name, dim=len(vector), vector=vector))
    _metrics["embed_batch"] += len(out)
    return out


@app.post("/v1/analyze", response_model=AnalyzeResponse)
async def analyze(
    image: UploadFile = File(...),
    mode: str = Form("multi"),
    top_k: int = Form(5),
) -> AnalyzeResponse:
    started = time.perf_counter()
    async with _semaphore:
        img = await _load_image(image)
        boxes = await asyncio.to_thread(_detector.detect, img)
        items: list[AnalyzeItem] = []
        vectors: list[list[float]] = []
        for box in boxes:
            crop = _crop(img, box)
            vector = await asyncio.to_thread(_embedder.embed, crop)
            vectors.append(vector)
            items.append(
                AnalyzeItem(
                    box=box,
                    vector=vector,
                    candidates=[],
                )
            )
        groups = group_by_similarity(vectors, list(range(len(vectors)))) if mode == "multi" else []
    _metrics["analyze"] += 1
    return AnalyzeResponse(
        model={
            "detector": _detector.name,
            "embedder": _embedder.name,
            "dim": _embedder.dim,
            "mode": mode,
            "top_k": top_k,
        },
        elapsed_ms=int((time.perf_counter() - started) * 1000),
        items=items,
        groups=groups,
    )


@app.post("/v1/segment", response_model=SegmentResponse)
async def segment(image: UploadFile = File(...)) -> SegmentResponse:
    started = time.perf_counter()
    async with _semaphore:
        img = await _load_image(image)
        boxes = await asyncio.to_thread(_detector.detect, img)
        crops = [_data_url(_crop(img, box)) for box in boxes]
    _metrics["segment"] += 1
    return SegmentResponse(
        model={"detector": _detector.name, "version": settings.version},
        elapsed_ms=int((time.perf_counter() - started) * 1000),
        boxes=boxes,
        crops=crops,
    )


@app.post("/v1/ocr")
async def ocr(image: UploadFile = File(...)) -> dict:
    img = await _load_image(image)
    return {"items": run_ocr(img)}


@app.get("/metrics", response_class=PlainTextResponse)
async def metrics() -> str:
    lines = [
        "# HELP goodser_vision_requests_total Total requests by endpoint",
        "# TYPE goodser_vision_requests_total counter",
    ]
    for key, value in sorted(_metrics.items()):
        lines.append(f'goodser_vision_requests_total{{endpoint="{key}"}} {value}')
    lines.append(f"goodser_vision_uptime_seconds {int(time.time() - _started_at)}")
    return "\n".join(lines) + "\n"
