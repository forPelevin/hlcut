SHELL := /bin/bash

IMAGE := hlcut-env:local

.PHONY: env_up setup test itest fmt

env_up:
	docker build -t $(IMAGE) -f docker/env/Dockerfile .

setup: env_up
	# Build whisper.cpp + download model into .cache (not tracked)
	docker run --rm \
		-v "$(PWD)":/work \
		-w /work \
		$(IMAGE) \
		bash -c "./scripts/setup.sh"

# Unit tests (table-driven; mocks allowed)

test: env_up
	docker run --rm \
		-v "$(PWD)":/work \
		-w /work \
		$(IMAGE) \
		bash -c "go test ./..."

# Integration tests (real deps + internet + OpenRouter)

itest: env_up
	@if [ ! -f .env ]; then echo "Missing .env (copy from .env.example)"; exit 2; fi
	@if ! grep -q '^OPENROUTER_API_KEY=' .env || grep -q '^OPENROUTER_API_KEY=$' .env; then echo "OPENROUTER_API_KEY is required in .env"; exit 2; fi
	docker run --rm \
		--env-file .env \
		-v "$(PWD)":/work \
		-w /work \
		$(IMAGE) \
		bash -c "go test -tags=integration ./..."

fmt: env_up
	docker run --rm \
		-v "$(PWD)":/work \
		-w /work \
		$(IMAGE) \
		bash -c "gofmt -w ."
