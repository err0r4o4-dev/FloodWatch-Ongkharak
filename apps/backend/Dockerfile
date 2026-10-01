FROM golang:1.26-alpine AS build

WORKDIR /src
COPY . ./
RUN go mod download

FROM build AS api-build
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM build AS worker-build
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker

FROM build AS bootstrap-build
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/bootstrap-admin ./cmd/bootstrap-admin

FROM alpine:3.21 AS api
WORKDIR /app
COPY --from=api-build /out/api /app/api
USER 65532:65532
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=3 CMD wget -q -O - http://127.0.0.1:8080/ready > /dev/null || exit 1
ENTRYPOINT ["/app/api"]

FROM alpine:3.21 AS worker
WORKDIR /app
COPY --from=worker-build /out/worker /app/worker
USER 65532:65532
ENTRYPOINT ["/app/worker"]

FROM alpine:3.21 AS admin-bootstrap
WORKDIR /app
COPY --from=bootstrap-build /out/bootstrap-admin /app/bootstrap-admin
USER 65532:65532
ENTRYPOINT ["/app/bootstrap-admin"]
