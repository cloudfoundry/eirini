ARG stack
ARG go_version=1.9.6
FROM golang:$go_version as lifecycle
ARG diego_version=2.7.0
ARG diego_repo=github.com/cloudfoundry/diego-release
ARG bal_repo=code.cloudfoundry.org/buildpackapplifecycle

WORKDIR /diego
RUN git clone --single-branch "https://${diego_repo}" . && \
  git checkout "v${diego_version}" && \
  git submodule update --init --recursive \
    src/code.cloudfoundry.org/archiver \
    src/code.cloudfoundry.org/buildpackapplifecycle \
    src/code.cloudfoundry.org/bytefmt \
    src/code.cloudfoundry.org/cacheddownloader \
    src/code.cloudfoundry.org/goshims \
    src/code.cloudfoundry.org/lager \
    src/code.cloudfoundry.org/systemcerts \
    src/github.com/cloudfoundry-incubator/credhub-cli \
    src/gopkg.in/yaml.v2
RUN GOPATH=/diego CGO_ENABLED=0 go install -a -installsuffix static "${bal_repo}/..."

FROM golang:$go_version as packs
ARG packs_repo=github.com/buildpack/packs

COPY . "src/${packs_repo}"
RUN CGO_ENABLED=0 go install -a -installsuffix static "${packs_repo}/cf/cmd/..."

FROM cloudfoundry/${stack}
COPY --from=lifecycle /diego/bin /lifecycle
COPY --from=packs /go/bin /packs
WORKDIR /workspace