# vi syntax=Dockerfile
ARG ALPINE_IMAGE=mirror.gcr.io/library/alpine:3.21
FROM ${ALPINE_IMAGE} AS download
ARG TARGETARCH=amd64
ARG KUBECTL_VERSION=1.33.4
ARG GCLOUD_VERSION=516.0.0
RUN apk add --no-cache curl ca-certificates python3
WORKDIR /out
RUN curl -fsSL -o kubectl "https://dl.k8s.io/release/v${KUBECTL_VERSION}/bin/linux/${TARGETARCH}/kubectl" \
    && chmod +x kubectl
WORKDIR /opt
RUN arch="${TARGETARCH}" \
    && if [ "${TARGETARCH}" = "amd64" ]; then arch=x86_64; fi \
    && if [ "${TARGETARCH}" = "arm64" ]; then arch=arm; fi \
    && curl -fsSL "https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-cli-${GCLOUD_VERSION}-linux-${arch}.tar.gz" | tar xz \
    && /opt/google-cloud-sdk/install.sh --quiet --usage-reporting false --path-update false \
    && /opt/google-cloud-sdk/bin/gcloud components install gke-gcloud-auth-plugin --quiet

FROM ${ALPINE_IMAGE}
LABEL org.opencontainers.image.title="kubectl-gcp"
LABEL org.opencontainers.image.description="kubectl and gke-gcloud-auth-plugin for GKE, using the runner-injected kubeconfig."
LABEL nuon.co/experimental="true"
ARG GIT_COMMIT_SHA
ENV GIT_COMMIT_SHA=${GIT_COMMIT_SHA}
RUN apk add --no-cache ca-certificates python3
COPY --from=download /out/kubectl /usr/local/bin/kubectl
COPY --from=download /opt/google-cloud-sdk /opt/google-cloud-sdk
ENV PATH="/opt/google-cloud-sdk/bin:${PATH}"
ENTRYPOINT ["/usr/local/bin/kubectl"]
