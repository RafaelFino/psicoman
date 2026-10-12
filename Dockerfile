# Psicoman — imagem única com os dois binários (admin e portal).
#
# modernc.org/sqlite é CGO-free, então compilamos estático (CGO_ENABLED=0) e a
# imagem final é um Alpine enxuto. Qual binário rodar é escolhido pelo comando
# do serviço no docker-compose (psicoman-admin | psicoman-portal).

# ---- build ----
FROM golang:1.24-alpine AS build
WORKDIR /src

# Cache de dependências: baixa os módulos antes de copiar o código.
COPY go.mod go.sum ./
RUN go mod download

# Código e compilação estática dos dois binários.
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /out/psicoman-admin  ./cmd/admin \
 && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /out/psicoman-portal ./cmd/portal

# ---- runtime ----
FROM alpine:3.20
# wget (busybox) para o healthcheck; tzdata para o fuso America/Sao_Paulo.
RUN apk add --no-cache tzdata ca-certificates \
 && adduser -D -u 10001 psicoman
ENV TZ=America/Sao_Paulo

WORKDIR /app
COPY --from=build /out/psicoman-admin  /usr/local/bin/psicoman-admin
COPY --from=build /out/psicoman-portal /usr/local/bin/psicoman-portal
COPY deploy/config.docker.yaml /app/config.docker.yaml

# Dados (sqlite, ged, logs) ficam num volume montado em /data.
RUN mkdir -p /data/ged /data/logs && chown -R psicoman:psicoman /data /app
USER psicoman
VOLUME ["/data"]

# O comando concreto (admin ou portal) vem do docker-compose.
# Default: admin.
CMD ["psicoman-admin", "-config", "/app/config.docker.yaml"]
