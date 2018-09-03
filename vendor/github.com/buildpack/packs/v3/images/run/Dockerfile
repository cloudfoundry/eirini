FROM packs/v3

ARG port=8080

EXPOSE ${port}

RUN apt-get update && \
  apt-get install -y wget && \
  rm -rf /var/lib/apt/lists/*

HEALTHCHECK --interval=30s --timeout=1s --start-period=60s --retries=1 \
  CMD wget --quiet --tries=1 --spider http://localhost:${port}/ || exit 1

USER packs

ENTRYPOINT ["/packs/launcher"]