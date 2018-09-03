ARG stack
ARG go_version=1.9.6
FROM golang:$go_version as helpers

RUN go get github.com/GoogleCloudPlatform/docker-credential-gcr
RUN go get github.com/awslabs/amazon-ecr-credential-helper/ecr-login/cli/docker-credential-ecr-login
RUN go get github.com/Azure/acr-docker-credential-helper/src/docker-credential-acr

FROM packs/${stack}

ARG stack

COPY --from=helpers /go/bin /usr/local/bin

ENV PACK_STACK_NAME packs/${stack}:run
ENV PACK_USE_HELPERS true

# TODO: remove
ENV PACK_DROPLET_PATH ./droplet.tgz
ENV PACK_METADATA_PATH ./result.json

ENTRYPOINT ["/packs/exporter"]
