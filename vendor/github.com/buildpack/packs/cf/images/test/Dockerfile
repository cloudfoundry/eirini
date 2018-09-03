ARG stack
FROM packs/${stack}:build AS build
FROM packs/${stack}:export AS export
FROM packs/${stack}

ARG go_version=1.9.6
ARG docker_version=18.03.1-ce

COPY --from=build /var/lib/buildpacks /var/lib/buildpacks
COPY --from=export /usr/local/bin/docker-credential-* /usr/local/bin/

RUN curl -L "https://storage.googleapis.com/golang/go${go_version}.linux-amd64.tar.gz" | tar -C /usr/local -xz
RUN curl -L "https://download.docker.com/linux/static/stable/x86_64/docker-${docker_version}.tgz" | tar -C /usr/local/bin -xz --strip-components 1 docker/docker
ENV PATH /usr/local/go/bin:$PATH

RUN mkdir /go
ENV GOPATH /go

ENTRYPOINT ["go", "test"]
