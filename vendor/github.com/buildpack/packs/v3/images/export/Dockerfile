ARG go_version=1.10.3
ARG stack=packs/v3

FROM golang:$go_version as helpers

RUN go get github.com/GoogleCloudPlatform/docker-credential-gcr
RUN go get github.com/awslabs/amazon-ecr-credential-helper/ecr-login/cli/docker-credential-ecr-login
RUN go get github.com/Azure/acr-docker-credential-helper/src/docker-credential-acr

FROM ${stack}

COPY --from=helpers /go/bin /usr/local/bin

USER packs

ENV PACK_BP_GROUP_PATH ./group.toml
ENV PACK_METADATA_PATH ./metadata.toml

ENTRYPOINT ["/packs/exporter"]
