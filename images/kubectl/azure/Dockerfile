# vi syntax=Dockerfile
ARG ALPINE_IMAGE=mirror.gcr.io/library/alpine:3.21
FROM ${ALPINE_IMAGE} AS download
ARG TARGETARCH=amd64
ARG KUBECTL_VERSION=1.33.4
ARG KUBELOGIN_VERSION=0.2.9
RUN apk add --no-cache curl ca-certificates unzip
WORKDIR /out
RUN curl -fsSL -o kubectl "https://dl.k8s.io/release/v${KUBECTL_VERSION}/bin/linux/${TARGETARCH}/kubectl" \
    && chmod +x kubectl
RUN curl -fsSL -o kubelogin.zip \
    "https://github.com/Azure/kubelogin/releases/download/v${KUBELOGIN_VERSION}/kubelogin-linux-${TARGETARCH}.zip" \
    && unzip -j kubelogin.zip "bin/linux_${TARGETARCH}/kubelogin" \
    && chmod +x kubelogin \
    && rm kubelogin.zip

FROM ${ALPINE_IMAGE}
LABEL org.opencontainers.image.title="kubectl-azure"
LABEL org.opencontainers.image.description="kubectl and kubelogin for AKS, using the runner-injected kubeconfig."
LABEL nuon.co/experimental="true"
ARG GIT_COMMIT_SHA
ENV GIT_COMMIT_SHA=${GIT_COMMIT_SHA}
RUN apk add --no-cache ca-certificates
COPY --from=download /out/kubectl /usr/local/bin/kubectl
COPY --from=download /out/kubelogin /usr/local/bin/kubelogin
ENTRYPOINT ["/usr/local/bin/kubectl"]
