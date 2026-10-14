FROM golang:1.26.1-bookworm AS go
FROM node:22.20.0-bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends git ca-certificates python3 python-is-python3 python3-venv gcc libc6-dev make && rm -rf /var/lib/apt/lists/* \
	&& git config --system --add safe.directory /workspace \
	&& git config --system core.checkStat minimal
COPY --from=go /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:${PATH}"
USER node
WORKDIR /workspace
