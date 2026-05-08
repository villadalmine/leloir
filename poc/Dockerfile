FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod .
COPY main.go .
COPY static/ static/
RUN go build -ldflags="-s -w" -o /poc .

FROM gcr.io/distroless/static-debian12:nonroot
LABEL org.opencontainers.image.source=https://github.com/villadalmine/leloir
COPY --from=builder /poc /poc
ENTRYPOINT ["/poc"]
