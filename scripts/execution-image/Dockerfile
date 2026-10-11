FROM golang:1.26.1-bookworm AS go
FROM node:22.20.0-bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends git ca-certificates python3 gcc libc6-dev && rm -rf /var/lib/apt/lists/*
COPY --from=go /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:${PATH}"
USER node
WORKDIR /workspace
