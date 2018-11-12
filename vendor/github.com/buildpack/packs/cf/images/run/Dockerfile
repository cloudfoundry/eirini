ARG stack
FROM packs/${stack}

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=1s --start-period=60s --retries=1 \
  CMD curl -f http://localhost:8080/ || exit 1

USER vcap

ENTRYPOINT ["/packs/launcher"]
