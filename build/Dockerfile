FROM openshift/origin-release:golang-1.15 as builder

ARG REMOTE_SOURCE
ARG REMOTE_SOURCE_DIR

COPY $REMOTE_SOURCE $REMOTE_SOURCE_DIR/app/
WORKDIR $REMOTE_SOURCE_DIR/app

RUN GOFLAGS="" go build -o build/_output/curator ./pkg/jobs/curator.go
RUN GOFLAGS="" go build -o build/_output/cluster-curator-controller ./pkg/controller/controller.go

FROM registry.access.redhat.com/ubi8/ubi-minimal:latest
RUN microdnf update

ARG IMAGE_DESCRIPTION
ARG IMAGE_NAME
ARG IMAGE_VERSION
ARG VCS_REF
ARG VCS_URL
ARG COMPONENT_NAME
ARG COMPONENT_VERSION
ARG REMOTE_SOURCE_DIR

ENV IMAGE_MANIFEST_PATH=/

ENV OPERATOR=/usr/local/bin/$COMPONENT_NAME \
    USER_UID=10001 \
    USER_NAME=$COMPONENT_NAME

COPY --from=builder $REMOTE_SOURCE_DIR/app/build/_output/curator .
COPY --from=builder $REMOTE_SOURCE_DIR/app/build/_output/cluster-curator-controller .
COPY --from=builder $REMOTE_SOURCE_DIR/app/build/bin/entrypoint /usr/local/bin
COPY --from=builder $REMOTE_SOURCE_DIR/app/build/bin/user_setup /usr/local/bin

LABEL org.label-schema.vendor="Red Hat" \
      org.label-schema.name="$IMAGE_NAME" \
      org.label-schema.description="$IMAGE_DESCRIPTION" \
      org.label-schema.vcs-ref="$VCS_REF" \
      org.label-schema.vcs-url="$VCS_URL" \
      org.label-schema.license="Red Hat Advanced Cluster Management for Kubernetes EULA" \
      org.label-schema.schema-version="1.0" \
      vendor="Red Hat" \
      name="$IMAGE_NAME" \
      summary="$IMAGE_DESCRIPTION" \
      description="$IMAGE_DESCRIPTION" \
      version="$IMAGE_VERSION" \
      release="$VCS_REF"
    
RUN  /usr/local/bin/user_setup

USER ${USER_UID}

ENTRYPOINT ["/usr/local/bin/entrypoint"]

# ARG REMOTE_SOURCE
# ARG REMOTE_SOURCE_DIR
# ARG GITHUB_TOKEN
# ARG COMPONENT_VERSION

# ENV USE_VENDORIZED_BUILD_HARNESS=true
# ENV COMPONENT_NAME=rcm-controller
# ENV COMPONENT_VERSION="${COMPONENT_VERSION}"
# ENV COMPONENT_TAG_EXTENSION=" "

# COPY $REMOTE_SOURCE $REMOTE_SOURCE_DIR/app/
# WORKDIR $REMOTE_SOURCE_DIR/app
# RUN git config --global url."https://${GITHUB_TOKEN}:x-oauth-basic@github.com/".insteadOf "https://github.com/"
# RUN GOFLAGS="" go build ./cmd/manager
# # Copy the Go Modules manifests
# COPY go.mod go.mod
# COPY go.sum go.sum
# # cache deps before building and copying source so that we don't need to re-download as much
# # and so that source changes don't invalidate our downloaded layer
# RUN go mod download

# # Copy the go source
# COPY main.go main.go
# COPY api/ api/
# COPY controllers/ controllers/

# # Build
# RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o manager main.go

# # Use distroless as minimal base image to package the manager binary
# # Refer to https://github.com/GoogleContainerTools/distroless for more details
# FROM gcr.io/distroless/static:nonroot
# WORKDIR /
# COPY --from=builder /workspace/manager .
# USER nonroot:nonroot

# ENTRYPOINT ["/manager"]
