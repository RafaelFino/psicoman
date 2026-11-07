# Psicoman — imagem do binário ÚNICO (cmd/psicoman), com o PocketBase embarcado.
#
# modernc.org/sqlite é CGO-free, então compilamos estático (CGO_ENABLED=0) e a
# imagem final é um Alpine enxuto. O processo roda `serve` do PocketBase; o
# HTTP/dir vêm dos flags (ver docker-compose.yml / docs/stack-local.md).

# ---- build ----
FROM golang:1.27-alpine AS build
WORKDIR /src

# Cache de dependências: baixa os módulos antes de copiar o código.
COPY go.mod go.sum ./
RUN go mod download

# Código e compilação estática do binário único.
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /out/psicoman ./cmd/psicoman

# ---- runtime ----
FROM alpine:3.20
# wget (busybox) para o healthcheck; tzdata para o fuso America/Sao_Paulo.
RUN apk add --no-cache tzdata ca-certificates \
 && adduser -D -u 10001 psicoman
ENV TZ=America/Sao_Paulo

WORKDIR /app
COPY --from=build /out/psicoman /usr/local/bin/psicoman

# Dados do PocketBase (SQLite, storage, migrations aplicadas no boot) em /data.
RUN mkdir -p /data && chown -R psicoman:psicoman /data /app
USER psicoman
VOLUME ["/data"]

EXPOSE 8090
# 9091: listener dedicado de métricas (só documental; sobe apenas com
# metrics.addr / PSICOMAN_METRICS_ADDR e NUNCA deve ser publicado na borda).
EXPOSE 9091
# serve do PocketBase, escutando em 0.0.0.0 dentro do container (atrás do
# Pangolin/compose). A config de cifra vem por env (PSICOMAN_CIPHER_*).
ENTRYPOINT ["psicoman"]
CMD ["serve", "--http", "0.0.0.0:8090", "--dir", "/data"]
