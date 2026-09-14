# vi syntax=Dockerfile
ARG ALPINE_IMAGE=mirror.gcr.io/library/alpine:3.21
FROM ${ALPINE_IMAGE} AS download
ARG TARGETARCH=amd64
ARG KUBECTL_VERSION=1.33.4
ARG AWS_IAM_AUTHENTICATOR_VERSION=0.6.30
RUN apk add --no-cache curl ca-certificates
WORKDIR /out
RUN curl -fsSL -o kubectl "https://dl.k8s.io/release/v${KUBECTL_VERSION}/bin/linux/${TARGETARCH}/kubectl" \
    && chmod +x kubectl
RUN curl -fsSL -o aws-iam-authenticator \
    "https://github.com/kubernetes-sigs/aws-iam-authenticator/releases/download/v${AWS_IAM_AUTHENTICATOR_VERSION}/aws-iam-authenticator_${AWS_IAM_AUTHENTICATOR_VERSION}_linux_${TARGETARCH}" \
    && chmod +x aws-iam-authenticator

FROM ${ALPINE_IMAGE}
LABEL org.opencontainers.image.title="kubectl-aws"
LABEL org.opencontainers.image.description="kubectl and aws-iam-authenticator for EKS, using the runner-injected kubeconfig."
LABEL nuon.co/experimental="true"
ARG GIT_COMMIT_SHA
ENV GIT_COMMIT_SHA=${GIT_COMMIT_SHA}
RUN apk add --no-cache ca-certificates
COPY --from=download /out/kubectl /usr/local/bin/kubectl
COPY --from=download /out/aws-iam-authenticator /usr/local/bin/aws-iam-authenticator
ENTRYPOINT ["/usr/local/bin/kubectl"]
