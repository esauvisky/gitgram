FROM golang:1.27-alpine AS build
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags="-s -w -X github.com/esauvisky/gitgram/internal/ops.Version=${VERSION}" \
    -o /gitgram ./cmd/gitgram \
    && mkdir -p /data

FROM gcr.io/distroless/static-debian12:nonroot
LABEL org.opencontainers.image.source="https://github.com/esauvisky/gitgram" \
      org.opencontainers.image.description="GitLab webhooks → Telegram cards that edit themselves" \
      org.opencontainers.image.licenses="MIT"
COPY --from=build /gitgram /gitgram
COPY --from=build --chown=nonroot:nonroot /data /data
VOLUME ["/data"]
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/gitgram", "healthcheck", "--url", "http://127.0.0.1:8080/healthz"]
ENTRYPOINT ["/gitgram"]
CMD ["serve"]
