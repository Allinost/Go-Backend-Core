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
	docker compose build

docker-up:
	docker compose up -d

docker-down:
	docker compose down
