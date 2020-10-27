FROM golang:1.15.3 as builder
WORKDIR /eirini/
COPY . .
RUN  CGO_ENABLED=0 GOOS=linux go build -mod vendor -trimpath -a -installsuffix cgo -o task-reporter ./cmd/task-reporter
ARG GIT_SHA
RUN if [ -z "$GIT_SHA" ]; then echo "GIT_SHA not set"; exit 1; else : ; fi

FROM scratch
COPY --from=builder /eirini/task-reporter /usr/local/bin/task-reporter
USER 1001
ENTRYPOINT [ "/usr/local/bin/task-reporter", \
        "--config", \
        "/etc/eirini/config/task-reporter.yml" \
]
ARG GIT_SHA
LABEL org.opencontainers.image.revision=$GIT_SHA \
      org.opencontainers.image.source=https://code.cloudfoundry.org/eirini
