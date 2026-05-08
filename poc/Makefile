IMAGE ?= ghcr.io/villadalmine/leloir-poc
TAG   ?= latest

.PHONY: build push import run

## Construye la imagen Docker
build:
	podman build -t $(IMAGE):$(TAG) .

## Pushea a ghcr.io (requiere: podman login ghcr.io -u villadalmine)
push: build
	podman push $(IMAGE):$(TAG)

## Importa la imagen directamente en k3s (sin registry, para pruebas locales)
import: build
	podman save $(IMAGE):$(TAG) | sudo podman exec -i k3s-server ctr images import -

## Corre localmente (requiere port-forward de Holmes en :8081)
run:
	HOLMES_URL=http://localhost:8081 LISTEN_ADDR=:3001 STATIC_DIR=static go run ./cmd/poc
