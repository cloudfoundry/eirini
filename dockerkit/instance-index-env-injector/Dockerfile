# syntax = docker/dockerfile:experimental

ARG baseimage=cloudfoundry/run:tiny

FROM golang:1.15 as builder
WORKDIR /eirini/
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -mod vendor -trimpath -installsuffix cgo -o instance-index-env-injector ./cmd/instance-index-env-injector
ARG GIT_SHA
RUN if [ -z "$GIT_SHA" ]; then echo "GIT_SHA not set"; exit 1; else : ; fi

FROM ${baseimage}
COPY --from=builder /eirini/instance-index-env-injector /usr/local/bin/instance-index-env-injector
USER 1001
ENTRYPOINT [ "/usr/local/bin/instance-index-env-injector", \
        "--config", \
        "/etc/eirini/config/instance-index-env-injector.yml" \
]
ARG GIT_SHA
LABEL org.opencontainers.image.revision=$GIT_SHA \
      org.opencontainers.image.source=https://code.cloudfoundry.org/eirini
