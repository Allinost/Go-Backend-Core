from __future__ import annotations

import math
from typing import Any


def cosine(a: list[float], b: list[float]) -> float:
    if not a or not b:
        return 0.0
    n = min(len(a), len(b))
    dot = sum(a[i] * b[i] for i in range(n))
    na = math.sqrt(sum(v * v for v in a[:n]))
    nb = math.sqrt(sum(v * v for v in b[:n]))
    if na == 0 or nb == 0:
        return 0.0
    return dot / (na * nb)


def group_by_similarity(
    vectors: list[list[float]],
    indexes: list[int],
    threshold: float = 0.86,
) -> list[dict[str, Any]]:
    groups: list[dict[str, Any]] = []
    for vec, index in zip(vectors, indexes):
        placed = False
        for group in groups:
            if cosine(vec, group["centroid"]) >= threshold:
                group["item_indexes"].append(index)
                group["vectors"].append(vec)
                size = len(group["vectors"])
                group["centroid"] = [
                    sum(v[i] for v in group["vectors"]) / size
                    for i in range(min(len(v) for v in group["vectors"]))
                ]
                placed = True
                break
        if not placed:
            groups.append({"item_indexes": [index], "centroid": vec[:], "vectors": [vec]})
    out: list[dict[str, Any]] = []
    for group in groups:
        out.append(
            {
                "item_indexes": group["item_indexes"],
                "quantity": len(group["item_indexes"]),
                "score": 1.0,
            }
        )
    return out
