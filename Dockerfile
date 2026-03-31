# Stage 1: Build frontend
FROM node:22-alpine AS frontend
WORKDIR /app
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ .
RUN npm run build

# Stage 2: Build Go binary
FROM golang:1.24-alpine AS backend
RUN apk add --no-cache git
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/dist /src/frontend/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /bangou .

# Stage 3: Runtime
FROM alpine:3.19
RUN apk add --no-cache mkvtoolnix su-exec
COPY --from=backend /bangou /usr/local/bin/bangou
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
