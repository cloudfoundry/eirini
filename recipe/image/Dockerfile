ARG baseimage=packs/cf
FROM ${baseimage}

WORKDIR /workspace

RUN mkdir -p /packs

COPY builder /packs/
COPY recipe /packs/

ENTRYPOINT [ \
  "/packs/recipe" \
]

