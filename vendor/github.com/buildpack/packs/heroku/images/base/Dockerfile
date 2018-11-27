ARG stack
FROM heroku/${stack}-build

ARG go_version=1.9.4
ARG diego_version=1.32.0

RUN useradd -ms /bin/bash -d /app heroku

RUN apt-get update -y
RUN apt-get install -y jq

RUN \
  curl -L "https://storage.googleapis.com/golang/go${go_version}.linux-amd64.tar.gz" | tar -C /usr/local -xz && \
  git -C /tmp clone --single-branch https://github.com/cloudfoundry/diego-release && \
  cd /tmp/diego-release && \
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
    src/gopkg.in/yaml.v2 && \
  export PATH=/usr/local/go/bin:$PATH && \
  export GOPATH=/tmp/diego-release && \
  CGO_ENABLED=0 go build -a -installsuffix static -o /lifecycle/builder code.cloudfoundry.org/buildpackapplifecycle/builder && \
  CGO_ENABLED=0 go build -a -installsuffix static -o /lifecycle/launcher code.cloudfoundry.org/buildpackapplifecycle/launcher && \
  CGO_ENABLED=0 go build -a -installsuffix static -o /lifecycle/shell code.cloudfoundry.org/buildpackapplifecycle/shell/shell && \
  rm -rf /tmp/diego-release /usr/local/go

