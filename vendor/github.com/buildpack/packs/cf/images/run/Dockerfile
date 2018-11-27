ARG stack
FROM packs/${stack}

ARG port=8080

EXPOSE ${port}

HEALTHCHECK --interval=30s --timeout=1s --start-period=60s --retries=1 \
  CMD curl -f http://localhost:${port}/ || exit 1

USER vcap

ENTRYPOINT ["/packs/launcher"]
