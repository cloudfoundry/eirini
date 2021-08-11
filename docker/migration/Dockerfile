# syntax = docker/dockerfile:experimental

ARG baseimage=scratch

FROM golang:1.16.7 as builder
WORKDIR /eirini/
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -mod vendor -trimpath -installsuffix cgo -o migration ./cmd/migration/
ARG GIT_SHA
RUN if [ -z "$GIT_SHA" ]; then echo "GIT_SHA not set"; exit 1; else : ; fi

FROM ${baseimage}
COPY --from=builder /eirini/migration /usr/local/bin/migration
USER 1001
ENTRYPOINT [ "/usr/local/bin/migration" ]
ARG GIT_SHA
LABEL org.opencontainers.image.revision=$GIT_SHA \
      org.opencontainers.image.source=https://code.cloudfoundry.org/eirini
