ARG go_version=1.10.1

FROM golang:${go_version} as deps

RUN go get github.com/sclevine/yj
RUN wget -qO bin/jq http://stedolan.github.io/jq/download/linux64/jq
RUN chmod +x bin/jq

FROM packs/v3:detect

USER root

RUN apt-get update && \
  apt-get install -y wget xz-utils && \
  rm -rf /var/lib/apt/lists/*

COPY --from=deps /go/bin /usr/local/bin

RUN mkdir -p /cache /launch/app /platform/env
RUN chown -R packs:packs /launch /cache

USER packs

ENV PACK_METADATA_PATH ./metadata.toml

ENTRYPOINT ["/packs/builder"]
