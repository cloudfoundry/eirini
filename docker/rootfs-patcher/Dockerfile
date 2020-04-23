ARG baseimage=scratch

FROM golang:1.13 as builder
WORKDIR /eirini/
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -mod vendor -a -trimpath -installsuffix cgo -o rootfs-patcher ./cmd/rootfs-patcher/
ARG GIT_SHA
RUN if [ -z "$GIT_SHA" ]; then echo "GIT_SHA not set"; exit 1; else : ; fi

FROM ${baseimage}

COPY --from=builder /eirini/rootfs-patcher /rootfs-patcher
USER 1001

ARG GIT_SHA
LABEL org.opencontainers.image.revision=$GIT_SHA \
      org.opencontainers.image.source=https://code.cloudfoundry.org/eirini
