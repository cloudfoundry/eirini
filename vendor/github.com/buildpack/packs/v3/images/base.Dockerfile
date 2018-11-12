ARG base=ubuntu:18.04
ARG go_version=1.10.3

FROM golang:${go_version} as builder
ARG lifecycle_ref=1e60998166197ab9697980edc2dc5c9d75628689
ARG lifecycle_repo=github.com/buildpack/lifecycle

WORKDIR /go/src/${lifecycle_repo}
RUN git clone "https://${lifecycle_repo}" . && git checkout "${lifecycle_ref}"
RUN CGO_ENABLED=0 go install -a -installsuffix static "${lifecycle_repo}/cmd/..."

RUN mv /go/bin /packs && mkdir /go/bin

RUN go get github.com/sclevine/yj

FROM ${base}
ARG jq_url=http://stedolan.github.io/jq/download/linux64/jq

RUN apt-get update && \
  apt-get install -y curl wget xz-utils ca-certificates && \
  rm -rf /var/lib/apt/lists/*

RUN useradd -u 1000 -mU -s /bin/bash packs

COPY --from=builder /packs /packs
COPY --from=builder /go/bin /usr/local/bin

RUN wget -qO /usr/local/bin/jq "${jq_url}" && chmod +x /usr/local/bin/jq

WORKDIR /workspace
RUN chown -R packs /workspace
