FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY app/ app/
# Versión que reporta /health; el pipeline de CD pasa el commit.
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -p 1 -trimpath -ldflags "-s -w -X main.version=${VERSION}" \
    -o /out/inventario ./app

FROM alpine:3.22
WORKDIR /app
COPY --from=build /out/inventario /app/inventario
COPY docs/openapi.yaml docs/openapi.yaml
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/app/inventario"]
