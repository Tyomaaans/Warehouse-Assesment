FROM golang:1.26.4-alpine AS build
WORKDIR /app
ENV CGO_ENABLED=0
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build go build -trimpath -ldflags="-s -w" -o /warehouse .

FROM alpine:3.22
RUN apk add --no-cache ca-certificates
COPY --from=build /warehouse /usr/local/bin/warehouse
USER 10001:10001
ENV APP_PORT=8080
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=5s --start-period=10s --retries=3 CMD ["warehouse", "healthcheck"]
ENTRYPOINT ["warehouse"]
CMD ["serve"]
