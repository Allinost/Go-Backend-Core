#!/usr/bin/env python3
"""GPU / PyTorch / OCR compatibility check for goodser-vision.

Usage:
  python scripts/check_runtime.py
  python scripts/check_runtime.py --strict

--strict exits non-zero when CUDA is expected but unavailable.
"""

from __future__ import annotations

import argparse
import os
import platform
import sys


def _print_version(module_name: str) -> None:
    try:
        module = __import__(module_name)
    except Exception as exc:  # noqa: BLE001
        print(f"{module_name}: NOT INSTALLED ({exc})")
        return
    version = getattr(module, "__version__", "unknown")
    print(f"{module_name}: {version}")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--strict", action="store_true")
    args = parser.parse_args()

    expected = os.getenv("DEVICE", "cpu").lower() == "cuda"
    print("== Goodser Vision runtime check ==")
    print("python:", sys.version.split()[0])
    print("platform:", platform.platform())
    print("DEVICE:", os.getenv("DEVICE", "cpu"))
    print("OCR_DEVICE:", os.getenv("GOODSER_VISION_OCR_DEVICE", "gpu"))
    print("EMBEDDER:", os.getenv("GOODSER_VISION_EMBEDDER", "fallback"))
    print("DETECTOR:", os.getenv("GOODSER_VISION_DETECTOR", "fallback"))
    print()

    torch = None
    try:
        import torch as _torch

        torch = _torch
        print("torch:", torch.__version__)
        print("torch cuda build:", torch.version.cuda)
        print("cudnn:", torch.backends.cudnn.version())
        print("cuda available:", torch.cuda.is_available())
        if torch.cuda.is_available():
            print("device name:", torch.cuda.get_device_name(0))
            capability = torch.cuda.get_device_capability(0)
            print("device capability:", capability)
            arch_list = getattr(torch.cuda, "get_arch_list", lambda: [])()
            print("torch arch list:", arch_list)
            if capability[0] >= 12 and "sm_120" not in arch_list:
                print("WARNING: GPU is Blackwell (sm_120) but this torch build has no sm_120 kernel")
        elif expected and args.strict:
            print("ERROR: DEVICE=cuda but CUDA is not available")
            return 1
    except Exception as exc:  # noqa: BLE001
        print("torch: NOT INSTALLED or broken:", exc)
        if expected and args.strict:
            return 1

    print()
    _print_version("torchvision")
    _print_version("transformers")
    _print_version("ultralytics")
    _print_version("paddle")
    _print_version("paddleocr")

    print()
    print("== done ==")
    return 0


if __name__ == "__main__":
    sys.exit(main())
