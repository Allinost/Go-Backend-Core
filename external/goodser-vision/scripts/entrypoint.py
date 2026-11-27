"""容器入口：直接用官方 pytorch 镜像运行，不派生镜像。

依赖安装到挂载目录 <项目>/deps/site-packages，通过 PYTHONPATH 生效；
torch 直接用镜像自带的，不重复下载。模型和缓存也都落在挂载目录里，
所以容器重建后无需重新安装，也不需要 docker build。
"""

from __future__ import annotations

import hashlib
import os
import pathlib
import subprocess
import sys

APP = pathlib.Path(__file__).resolve().parent.parent
DEPS = pathlib.Path(os.environ.get("GOODSER_VISION_DEPS_DIR", str(APP / "deps")))
TARGET = DEPS / "site-packages"
MARKER = DEPS / ".installed.sha256"


def resolve_requirements(value: str) -> pathlib.Path:
    path = pathlib.Path(value)
    return path if path.is_absolute() else APP / path


def pip_install(requirements: pathlib.Path) -> None:
    subprocess.check_call(
        [
            sys.executable,
            "-m",
            "pip",
            "install",
            "--no-cache-dir",
            "--upgrade",
            "--target",
            str(TARGET),
            "-r",
            str(requirements),
        ]
    )


def ensure_system_libs() -> None:
    """官方镜像可能缺 libGL/libglib，运行时补装（不写镜像层，容器重建后重装一次）。"""
    import ctypes.util
    import shutil

    if ctypes.util.find_library("GL") and ctypes.util.find_library("glib-2.0"):
        return
    apt = shutil.which("apt-get")
    if not apt:
        print("[entrypoint] 缺少 libGL 且没有 apt-get，OpenCV/PaddleOCR 可能不可用", flush=True)
        return
    try:
        print("[entrypoint] 补装系统库 libgl1/libglib2.0-0/libgomp1", flush=True)
        subprocess.check_call([apt, "update", "-qq"])
        subprocess.check_call(
            [apt, "install", "-y", "--no-install-recommends", "libgl1", "libglib2.0-0", "libgomp1"]
        )
    except subprocess.CalledProcessError as exc:
        print(f"[entrypoint] 系统库安装失败（可忽略，OCR 可能不可用）: {exc}", flush=True)


def main() -> None:
    if not (APP / "app" / "main.py").is_file():
        raise SystemExit(
            f"[entrypoint] 找不到 {APP / 'app' / 'main.py'}；"
            "请在仓库的 external/goodser-vision 目录执行 docker compose"
        )
    main_req = resolve_requirements("requirements-real.txt")
    paddle_req = resolve_requirements(
        os.environ.get("PADDLE_REQUIREMENTS", "requirements-real-gpu.txt")
    )
    for path in (main_req, paddle_req):
        if not path.is_file():
            raise SystemExit(f"[entrypoint] 缺少依赖清单: {path}")

    TARGET.mkdir(parents=True, exist_ok=True)
    ensure_system_libs()

    digest = hashlib.sha256()
    for path in (main_req, paddle_req):
        digest.update(path.read_bytes())
    wanted = digest.hexdigest()
    installed = MARKER.read_text(encoding="utf-8").strip() if MARKER.is_file() else ""

    if installed != wanted:
        print("[entrypoint] 首次启动或依赖清单变更，开始安装依赖（只需一次）", flush=True)
        pip_install(main_req)
        pip_install(paddle_req)
        MARKER.write_text(wanted + "\n", encoding="utf-8")
        print("[entrypoint] 依赖安装完成", flush=True)
    else:
        print("[entrypoint] 依赖已是最新，跳过安装", flush=True)

    env = dict(os.environ)
    existing = env.get("PYTHONPATH", "")
    env["PYTHONPATH"] = str(TARGET) + (os.pathsep + existing if existing else "")

    os.chdir(APP)
    os.execve(
        sys.executable,
        [sys.executable, "-m", "uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8000"],
        env,
    )


if __name__ == "__main__":
    main()
