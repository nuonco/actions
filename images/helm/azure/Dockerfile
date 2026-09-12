# vi syntax=Dockerfile
ARG ALPINE_IMAGE=mirror.gcr.io/library/alpine:3.21
FROM ${ALPINE_IMAGE} AS download
ARG TARGETARCH=amd64
ARG HELM_VERSION=4.3.0
ARG KUBELOGIN_VERSION=0.2.9
RUN apk add --no-cache curl ca-certificates unzip
WORKDIR /out
RUN curl -fsSL "https://get.helm.sh/helm-v${HELM_VERSION}-linux-${TARGETARCH}.tar.gz" | tar xz \
    && mv linux-${TARGETARCH}/helm helm \
    && chmod +x helm \
    && rm -r linux-${TARGETARCH}
RUN curl -fsSL -o kubelogin.zip \
    "https://github.com/Azure/kubelogin/releases/download/v${KUBELOGIN_VERSION}/kubelogin-linux-${TARGETARCH}.zip" \
    && unzip -j kubelogin.zip "bin/linux_${TARGETARCH}/kubelogin" \
    && chmod +x kubelogin \
    && rm kubelogin.zip

FROM ${ALPINE_IMAGE}
ARG HELM_VERSION=4.3.0
LABEL org.opencontainers.image.title="helm-azure"
LABEL org.opencontainers.image.description="helm and kubelogin for AKS, using the runner-injected kubeconfig."
LABEL org.opencontainers.image.version="${HELM_VERSION}"
LABEL nuon.co/experimental="true"
ARG GIT_COMMIT_SHA
ENV GIT_COMMIT_SHA=${GIT_COMMIT_SHA}
RUN apk add --no-cache ca-certificates
COPY --from=download /out/helm /usr/local/bin/helm
COPY --from=download /out/kubelogin /usr/local/bin/kubelogin
ENTRYPOINT ["/usr/local/bin/helm"]
