FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
RUN CGO_ENABLED=0 go build -p 1 -trimpath -o /out/inventario ./cmd/inventario

FROM alpine:3.22
WORKDIR /app
COPY --from=build /out/inventario /app/inventario
COPY api/openapi.yaml api/openapi.yaml
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/app/inventario"]
