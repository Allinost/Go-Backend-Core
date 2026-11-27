.PHONY: install run dev test lint docker-build docker-up docker-down check-gpu

check-gpu:
	python scripts/check_runtime.py

install:
	python -m pip install -r requirements.txt

run:
	uvicorn app.main:app --host 0.0.0.0 --port 8000

dev:
	uvicorn app.main:app --host 0.0.0.0 --port 8000 --reload

test:
	python -m pytest -q

docker-build:
	@echo "直接使用官方 pytorch 镜像，无需构建；docker compose up -d 即可"

docker-up:
	docker compose up -d

docker-down:
	docker compose down
