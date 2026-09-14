# vi syntax=Dockerfile
ARG ALPINE_IMAGE=mirror.gcr.io/library/alpine:3.21
FROM ${ALPINE_IMAGE} AS download
ARG TARGETARCH=amd64
ARG TERRAFORM_VERSION=1.16.2
ARG HELM_VERSION=4.3.0
ARG KUBECTL_VERSION=1.33.4
ARG GCLOUD_VERSION=516.0.0
RUN apk add --no-cache curl ca-certificates python3 unzip
WORKDIR /out
RUN curl -fsSL -o terraform.zip \
    "https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_linux_${TARGETARCH}.zip" \
    && unzip -j terraform.zip terraform \
    && chmod +x terraform \
    && rm terraform.zip
RUN curl -fsSL "https://get.helm.sh/helm-v${HELM_VERSION}-linux-${TARGETARCH}.tar.gz" | tar xz \
    && mv linux-${TARGETARCH}/helm helm \
    && chmod +x helm \
    && rm -r linux-${TARGETARCH}
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
ARG TERRAFORM_VERSION=1.16.2
LABEL org.opencontainers.image.title="terraform-gcp"
LABEL org.opencontainers.image.description="terraform, helm, kubectl, git, and gke-gcloud-auth-plugin for GKE, using the runner-injected kubeconfig."
LABEL org.opencontainers.image.version="${TERRAFORM_VERSION}"
LABEL nuon.co/experimental="true"
ARG GIT_COMMIT_SHA
ENV GIT_COMMIT_SHA=${GIT_COMMIT_SHA}
RUN apk add --no-cache ca-certificates git python3
COPY --from=download /out/terraform /usr/local/bin/terraform
COPY --from=download /out/helm /usr/local/bin/helm
COPY --from=download /out/kubectl /usr/local/bin/kubectl
COPY --from=download /opt/google-cloud-sdk /opt/google-cloud-sdk
ENV PATH="/opt/google-cloud-sdk/bin:${PATH}"
ENTRYPOINT ["/usr/local/bin/terraform"]
