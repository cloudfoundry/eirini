ARG baseimage=scratch

FROM golang:1.15 as builder
WORKDIR /eirini/
COPY . .
RUN  CGO_ENABLED=0 GOOS=linux go build -mod vendor -trimpath -a -installsuffix cgo -o metrics-collector ./cmd/metrics-collector/
ARG GIT_SHA
RUN if [ -z "$GIT_SHA" ]; then echo "GIT_SHA not set"; exit 1; else : ; fi

FROM ${baseimage}
COPY --from=builder /eirini/metrics-collector /usr/local/bin/metrics-collector
USER 1001
ENTRYPOINT [ "/usr/local/bin/metrics-collector", \
	"--config", \
	"/etc/eirini-metrics/config/metrics.yml" \
]
ARG GIT_SHA
LABEL org.opencontainers.image.revision=$GIT_SHA \
      org.opencontainers.image.source=https://code.cloudfoundry.org/eirini
