# ── Stage 1: build ────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /src
COPY go.mod ./
RUN go mod download

COPY cmd/ cmd/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/server ./cmd/poc

# ── Stage 2: runtime ──────────────────────────────────────────────────────────
FROM scratch

COPY --from=builder /out/server /server
COPY static/ /static/

WORKDIR /
ENV LISTEN_ADDR=:8080

EXPOSE 8080
ENTRYPOINT ["/server"]
