# Built from the repository root, which is the build context:
#   docker build -f docker/Dockerfile -t go-income-mgr:<tag> .

# Frontend stage: builds the Svelte app into /web/dist.
FROM node:24-alpine AS web

WORKDIR /web

# Install dependencies first so this layer is cached until package.json/package-lock.json change.
COPY web/package.json web/package-lock.json ./
RUN npm ci

COPY web/ .
RUN npm run build

# Build stage: compiles a static binary with the full Go toolchain.
FROM golang:1.27-alpine AS build

WORKDIR /src

# Download dependencies first so this layer is cached until go.mod/go.sum change.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# The compiled frontend is embedded in the binary (see web/embed.go).
COPY --from=web /web/dist ./web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

# Runtime stage: only the binary, no shell or package manager, runs as non-root.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/api /api

EXPOSE 8080
ENTRYPOINT ["/api"]
