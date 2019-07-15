FROM alpine:latest

RUN apk add --no-cache grep curl bash

RUN curl -o /usr/local/bin/kubectl -sSL https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl && chmod +x /usr/local/bin/kubectl

COPY never-tell-me-the-odds.sh /
RUN chmod +x never-tell-me-the-odds.sh

ENTRYPOINT [ "/never-tell-me-the-odds.sh" ]
