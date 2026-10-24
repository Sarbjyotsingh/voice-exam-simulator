# ---- build ----
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o /out/server .

# ---- runtime ----
FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 app \
    && mkdir -p /data/ledger && chown -R app /data
COPY --from=build /out/server /usr/local/bin/server
COPY --from=build /src/catalog /app/catalog
USER app
ENV LEDGER_DIR=/data/ledger PORT=8080 GOMEMLIMIT=480MiB GOGC=100
VOLUME /data
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s CMD ["server","-healthcheck"]
ENTRYPOINT ["server"]
