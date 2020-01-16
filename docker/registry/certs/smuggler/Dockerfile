FROM alpine:latest
ARG GIT_SHA
RUN if [ -z "$GIT_SHA" ]; then echo "GIT_SHA not set"; exit 1; else : ; fi

RUN apk add --no-cache grep curl bash

RUN curl -o /usr/local/bin/kubectl -sSL https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl && chmod +x /usr/local/bin/kubectl

COPY never-tell-me-the-odds.sh /
RUN chmod +x never-tell-me-the-odds.sh
USER 1001

ENTRYPOINT [ "/never-tell-me-the-odds.sh" ]
LABEL org.opencontainers.image.revision=$GIT_SHA \
      org.opencontainers.image.source=https://code.cloudfoundry.org/eirini
