FROM golang:1.13 as builder
WORKDIR /eirini/
COPY . .
RUN  CGO_ENABLED=0 GOOS=linux go build -mod vendor -trimpath -a -installsuffix cgo -o route-emitter ./cmd/route-emitter/
ARG GIT_SHA
RUN if [ -z "$GIT_SHA" ]; then echo "GIT_SHA not set"; exit 1; else : ; fi

FROM scratch
COPY --from=builder /eirini/route-emitter /workspace/jobs/router-emitter/bin/route-emitter
ENTRYPOINT [ "/workspace/jobs/route-emitter/bin/route-emitter", \
	"--config", \
	"/workspace/jobs/route-emitter/config/route-emitter.yml" \
]
ARG GIT_SHA
LABEL org.opencontainers.image.revision=$GIT_SHA \
      org.opencontainers.image.source=https://code.cloudfoundry.org/eirini
