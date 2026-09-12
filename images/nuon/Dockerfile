# vi syntax=Dockerfile
ARG DEBIAN_IMAGE=mirror.gcr.io/library/debian:bookworm-slim
ARG NUON_VERSION=0.19.1174
FROM ${DEBIAN_IMAGE} AS install
RUN apt-get update \
    && apt-get install -y --no-install-recommends bash ca-certificates curl \
    && rm -rf /var/lib/apt/lists/*
ARG NUON_VERSION
RUN set -eu; \
    export NUON_VERSION; \
    curl -fsSL https://nuon-artifacts.s3.us-west-2.amazonaws.com/cli/install.sh | bash -s -- --no-input

FROM ${DEBIAN_IMAGE}
ARG NUON_VERSION
LABEL org.opencontainers.image.title="nuon-cli"
LABEL org.opencontainers.image.description="Nuon CLI, pinned to the ctl-api promotion release."
LABEL org.opencontainers.image.version="${NUON_VERSION}"
LABEL nuon.co/experimental="true"
ARG GIT_COMMIT_SHA
ENV GIT_COMMIT_SHA=${GIT_COMMIT_SHA}
ENV NUON_NO_TTY=true
ENV CI=true
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*
COPY --from=install /usr/local/bin/nuon /usr/local/bin/nuon
ENTRYPOINT ["/usr/local/bin/nuon"]
