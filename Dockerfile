# --platform=$BUILDPLATFORM fija la etapa de compilación en la arquitectura DEL RUNNER, no la del
# destino: así Go cross-compila nativo y rápido, sin emulación QEMU.
FROM --platform=$BUILDPLATFORM golang:1.24 AS build
WORKDIR /src
# Copiamos primero go.mod (y go.sum si existe): esta capa solo se invalida si cambian dependencias.
COPY go.mod go.su[m] ./
RUN go mod download
COPY . .
ARG VERSION=dev
# Docker inyecta TARGETOS y TARGETARCH a partir del --platform que pide el build.
ARG TARGETOS
ARG TARGETARCH
# CGO_ENABLED=0 → binario estático, sin libc: corre en una imagen sin sistema operativo.
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" -o /out/api ./cmd/api

# Etapa final mínima. Sin shell, sin gestor de paquetes. No lleva ningún RUN, así que no hace falta
# emular la arquitectura destino para armarla.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/api /api
# UID numérico, no el nombre "nonroot": el kubelet no resuelve nombres y rechaza el pod con
# CreateContainerConfigError cuando el Deployment pide runAsNonRoot.
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/api"]
