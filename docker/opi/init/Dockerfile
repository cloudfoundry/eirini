FROM alpine:latest
ARG GIT_SHA
RUN if [ -z "$GIT_SHA" ]; then echo "GIT_SHA not set"; exit 1; else : ; fi

RUN apk add --no-cache bash curl jq

RUN curl --fail --silent --show-error --location --output /usr/bin/goml https://github.com/JulzDiverse/goml/releases/download/v0.4.0/goml-linux-amd64 && chmod +x /usr/bin/goml

COPY init.sh /
RUN chmod +x /init.sh

ENTRYPOINT [ "/bin/bash", "-c", "/init.sh" ]
LABEL org.opencontainers.image.revision=$GIT_SHA \
      org.opencontainers.image.source=https://code.cloudfoundry.org/eirini
