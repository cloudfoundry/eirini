# syntax = docker/dockerfile:experimental

ARG baseimage=cloudfoundry/run:tiny

FROM golang:1.19 as builder
WORKDIR /eirini/
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -mod vendor -trimpath -installsuffix cgo -o eirini ./cmd/api/
ARG GIT_SHA
RUN if [ -z "$GIT_SHA" ]; then echo "GIT_SHA not set"; exit 1; else : ; fi

FROM ${baseimage}
COPY --from=builder /eirini/eirini /usr/local/bin/api
USER 1001
ENTRYPOINT [ "/usr/local/bin/api" ]
ARG GIT_SHA
LABEL org.opencontainers.image.revision=$GIT_SHA \
      org.opencontainers.image.source=https://code.cloudfoundry.org/eirini
