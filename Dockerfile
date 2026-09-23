# Build the browser application before compiling Go, because the production
# binary embeds frontend/dist.
FROM node:22.14.0-bookworm-slim AS frontend-build

WORKDIR /src/frontend

COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

COPY frontend/ ./
RUN npm run build

# Build CGO SQLite on Debian so the runtime uses the same libc family.
FROM golang:1.27.0-bookworm AS go-build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./
COPY --from=frontend-build /src/frontend/dist ./frontend/dist
RUN CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o /out/hooklook .

FROM debian:bookworm-slim

ARG APP_UID=10001
ARG APP_GID=10001

RUN groupadd --gid "${APP_GID}" hooklook \
    && useradd --uid "${APP_UID}" --gid hooklook --home-dir /data --create-home \
        --shell /usr/sbin/nologin hooklook

WORKDIR /data

COPY --from=go-build --chown=hooklook:hooklook /out/hooklook /usr/local/bin/hooklook

USER hooklook:hooklook

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/hooklook"]
