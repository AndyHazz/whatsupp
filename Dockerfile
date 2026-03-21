# ============================================================
# Stage 1: Build frontend
# ============================================================
FROM node:20-alpine AS frontend-build

WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# ============================================================
# Stage 2: Build Go binary
# ============================================================
FROM golang:1.24-alpine AS go-build

RUN apk add --no-cache gcc musl-dev

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

# Copy compiled frontend into the embed path
COPY --from=frontend-build /app/frontend/dist /app/internal/web/dist/

# Copy all Go source
COPY . .
# Overwrite any local dist with the freshly built one
COPY --from=frontend-build /app/frontend/dist /app/internal/web/dist/

RUN CGO_ENABLED=1 go build -o /whatsupp ./cmd/whatsupp

# ============================================================
# Stage 3: Minimal runtime
# ============================================================
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

COPY --from=go-build /whatsupp /usr/local/bin/whatsupp

RUN mkdir -p /data /etc/whatsupp

VOLUME ["/data"]
EXPOSE 8080

ENTRYPOINT ["whatsupp"]
CMD ["serve"]
