# vi syntax=Dockerfile
ARG GO_BUILDER_IMAGE=mirror.gcr.io/library/golang:1.27-alpine
FROM ${GO_BUILDER_IMAGE} AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /dns-checks .

FROM gcr.io/distroless/static-debian12
LABEL org.opencontainers.image.title="dns-checks"
LABEL org.opencontainers.image.description="Check public DNS delegation, SOA, CAA, and DS for a domain."
LABEL nuon.co/experimental="true"
ARG GIT_COMMIT_SHA
ENV GIT_COMMIT_SHA=${GIT_COMMIT_SHA}
COPY --from=builder /dns-checks /dns-checks
ENTRYPOINT ["/dns-checks"]
