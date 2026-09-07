FROM golang:1.26.4-alpine AS build
WORKDIR /src
ENV GOPROXY=https://proxy.golang.org,direct
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/api ./cmd/api && \
    CGO_ENABLED=0 go build -trimpath -o /out/travel ./cmd/travel && \
    CGO_ENABLED=0 go build -trimpath -o /out/planner ./cmd/planner && \
    CGO_ENABLED=0 go build -trimpath -o /out/admin ./cmd/admin
FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/ /app/bin/
COPY docs/openapi.yaml /app/docs/openapi.yaml
USER 65532:65532
CMD ["/app/bin/api"]
