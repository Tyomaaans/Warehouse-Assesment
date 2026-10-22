ARG BINARY_SOURCE=build
FROM golang:1.26.4-alpine@sha256:3ad57304ad93bbec8548a0437ad9e06a455660655d9af011d58b993f6f615648 AS build
WORKDIR /app
ENV CGO_ENABLED=0
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build go build -buildvcs=false -trimpath -ldflags="-s -w" -o /warehouse .

FROM scratch AS prebuilt
COPY compiled/api /warehouse

FROM ${BINARY_SOURCE} AS binary

FROM alpine:3.22@sha256:d406446bc05b82480756ce8f1287964d28768c23955fa9480d2a74a2eae7a654
RUN apk add --no-cache ca-certificates
COPY --from=binary --chmod=755 /warehouse /usr/local/bin/warehouse
USER 10001:10001
ENV APP_PORT=8080
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=5s --start-period=10s --retries=3 CMD ["warehouse", "healthcheck"]
ENTRYPOINT ["warehouse"]
CMD ["serve"]
