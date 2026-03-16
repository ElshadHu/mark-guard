FROM golang:1.25-alpine AS builder

WORKDIR /src

# cache dependencies separately from source code
COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags="-s -w -X github.com/ElshadHu/mark-guard/internal/cli.Version=${VERSION}" -o /mark-guard ./cmd/mark-guard

# runtime stage 
FROM alpine:3.21
RUN apk add --no-cache ca-certificates git
COPY --from=builder /mark-guard /usr/bin/mark-guard
ENTRYPOINT [ "mark-guard" ]
