FROM golang:1.12 as builder
WORKDIR /eirini/
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -mod vendor -a -installsuffix cgo -o rootfs-patcher ./cmd/rootfs-patcher/
ARG GIT_SHA
RUN if [ -z "$GIT_SHA" ]; then echo "GIT_SHA not set"; exit 1; else : ; fi

FROM scratch

COPY --from=builder /eirini/rootfs-patcher /rootfs-patcher
ARG GIT_SHA
LABEL org.opencontainers.image.revision=$GIT_SHA \
      org.opencontainers.image.source=https://code.cloudfoundry.org/eirini
