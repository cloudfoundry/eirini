ARG baseimage=scratch

FROM golang:1.13 as builder
WORKDIR /eirini/
COPY . .
RUN  CGO_ENABLED=0 GOOS=linux go build -mod vendor -trimpath -a -installsuffix cgo -o lrp-controller ./cmd/lrp-controller
ARG GIT_SHA
RUN if [ -z "$GIT_SHA" ]; then echo "GIT_SHA not set"; exit 1; else : ; fi

FROM ${baseimage}
COPY --from=builder /eirini/lrp-controller /usr/local/bin/lrp-controller
USER 1001
ENTRYPOINT [ "/usr/local/bin/lrp-controller", \
	"--config", \
	"/etc/eirini/config/lrp-controller.yml" \
]
ARG GIT_SHA
LABEL org.opencontainers.image.revision=$GIT_SHA \
      org.opencontainers.image.source=https://code.cloudfoundry.org/eirini
