FROM golang:1.13 as builder
WORKDIR /eirini/
COPY . .
RUN  CGO_ENABLED=0 GOOS=linux go build -mod vendor -trimpath -a -installsuffix cgo -o route-statefulset-informer ./cmd/route-statefulset-informer/
ARG GIT_SHA
RUN if [ -z "$GIT_SHA" ]; then echo "GIT_SHA not set"; exit 1; else : ; fi

FROM scratch
COPY --from=builder /eirini/route-statefulset-informer /usr/local/bin/route-statefulset-informer
USER 1001
ENTRYPOINT [ "/usr/local/bin/route-statefulset-informer", \
	"--config", \
	"/etc/eirini/routing.yml" \
]
ARG GIT_SHA
LABEL org.opencontainers.image.revision=$GIT_SHA \
      org.opencontainers.image.source=https://code.cloudfoundry.org/eirini
