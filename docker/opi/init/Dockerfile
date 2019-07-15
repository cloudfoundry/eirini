FROM alpine:latest

RUN apk add --no-cache bash curl jq

RUN curl --fail --silent --show-error --location --output /usr/bin/goml https://github.com/JulzDiverse/goml/releases/download/v0.4.0/goml-linux-amd64 && chmod +x /usr/bin/goml

COPY init.sh /
RUN chmod +x /init.sh

ENTRYPOINT [ "/bin/bash", "-c", "/init.sh" ]
