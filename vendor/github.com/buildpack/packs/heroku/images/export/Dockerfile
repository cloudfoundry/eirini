FROM gcr.io/cloud-builders/docker

ARG stack

RUN mkdir /workspace
WORKDIR /workspace

RUN mkdir /packs
RUN ( \
  echo "FROM packs/${stack}:run" && \
  echo 'ADD droplet.tgz /' && \
  echo 'ENTRYPOINT ["/packs/launcher"]' \
  ) > /packs/Dockerfile
COPY export /packs/

ENTRYPOINT ["/packs/export"]
