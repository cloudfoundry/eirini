# syntax = docker/dockerfile:experimental

ARG baseimage=scratch

FROM golang:1.15 as builder
WORKDIR /eirini/
COPY go.* .
RUN go mod download
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -mod vendor -trimpath -installsuffix cgo -o staging-reporter ./cmd/staging-reporter
ARG GIT_SHA
RUN if [ -z "$GIT_SHA" ]; then echo "GIT_SHA not set"; exit 1; else : ; fi

FROM ${baseimage}
COPY --from=builder /eirini/staging-reporter /usr/local/bin/staging-reporter
USER 1001
ENTRYPOINT [ "/usr/local/bin/staging-reporter", \
	"--config", \
	"/etc/eirini/config/staging-reporter.yml" \
]
ARG GIT_SHA
LABEL org.opencontainers.image.revision=$GIT_SHA \
      org.opencontainers.image.source=https://code.cloudfoundry.org/eirini
