# ---- Build stage ----
FROM golang:1.25-alpine AS build
WORKDIR /src

# Download dependencies first so this layer is cached between code changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

# ---- Runtime stage ----
# Distroless: no shell or package manager, runs as non-root. Small attack surface.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/api /out/migrate /app/
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/api"]
