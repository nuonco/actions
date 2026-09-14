# vi syntax=Dockerfile
ARG ALPINE_IMAGE=mirror.gcr.io/library/alpine:3.21
FROM ${ALPINE_IMAGE} AS download
ARG TARGETARCH=amd64
ARG HELM_VERSION=4.3.0
ARG GCLOUD_VERSION=516.0.0
RUN apk add --no-cache curl ca-certificates python3
WORKDIR /out
RUN curl -fsSL "https://get.helm.sh/helm-v${HELM_VERSION}-linux-${TARGETARCH}.tar.gz" | tar xz \
    && mv linux-${TARGETARCH}/helm helm \
    && chmod +x helm \
    && rm -r linux-${TARGETARCH}
WORKDIR /opt
RUN arch="${TARGETARCH}" \
    && if [ "${TARGETARCH}" = "amd64" ]; then arch=x86_64; fi \
    && if [ "${TARGETARCH}" = "arm64" ]; then arch=arm; fi \
    && curl -fsSL "https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-cli-${GCLOUD_VERSION}-linux-${arch}.tar.gz" | tar xz \
    && /opt/google-cloud-sdk/install.sh --quiet --usage-reporting false --path-update false \
    && /opt/google-cloud-sdk/bin/gcloud components install gke-gcloud-auth-plugin --quiet

FROM ${ALPINE_IMAGE}
ARG HELM_VERSION=4.3.0
LABEL org.opencontainers.image.title="helm-gcp"
LABEL org.opencontainers.image.description="helm and gke-gcloud-auth-plugin for GKE, using the runner-injected kubeconfig."
LABEL org.opencontainers.image.version="${HELM_VERSION}"
LABEL nuon.co/experimental="true"
ARG GIT_COMMIT_SHA
ENV GIT_COMMIT_SHA=${GIT_COMMIT_SHA}
RUN apk add --no-cache ca-certificates python3
COPY --from=download /out/helm /usr/local/bin/helm
COPY --from=download /opt/google-cloud-sdk /opt/google-cloud-sdk
ENV PATH="/opt/google-cloud-sdk/bin:${PATH}"
ENTRYPOINT ["/usr/local/bin/helm"]
