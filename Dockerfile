FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X main.buildVersion=${VERSION} -X main.buildCommit=${COMMIT} -X main.buildDate=${DATE}" \
    -o /fox-control ./cmd/fox-control

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /fox-control /usr/local/bin/fox-control
RUN mkdir -p /etc/fox-control /var/lib/fox-control && \
    chown 65532:65532 /var/lib/fox-control
USER 65532:65532
VOLUME ["/var/lib/fox-control"]
EXPOSE 9090
ENTRYPOINT ["fox-control"]
CMD ["serve", "--config", "/etc/fox-control/fox-control.toml"]
