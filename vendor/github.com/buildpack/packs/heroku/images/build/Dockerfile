ARG stack
FROM packs/${stack}

ARG buildpacks

WORKDIR /workspace

RUN mkdir -p /packs
RUN curl -o /packs/cytokine -L https://heroku-packs.s3.amazonaws.com/cytokine-a2a26fe7f9e1f05489e743fc55b863eb9079d94c
RUN chmod +x /packs/cytokine

RUN /packs/cytokine get-default-buildpacks \
  --language=ruby \
  --language=clojure \
  --language=python \
  --language=java \
  --language=gradle \
  --language=scala \
  --language=php \
  --language=go \
  --language=nodejs \
  /var/lib/buildpacks

COPY builder /packs/

ENTRYPOINT [ \
  "/packs/builder", \
  "-buildpacksDir", "/var/lib/buildpacks", \
  "-appDir", "/tmp/app", \
  "-cacheDir", "/tmp/cache", \
  "-envDir", "/tmp/env", \
  "-outputSlug", "/out/slug.tgz", \
  "-outputCache", "/cache/cache.tgz" \
]
