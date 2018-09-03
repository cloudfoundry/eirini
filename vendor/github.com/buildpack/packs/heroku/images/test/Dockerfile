ARG stack
FROM packs/${stack}

ARG go_version=1.9.4

RUN mkdir /go

ENV GOPATH /go
ENV PATH /usr/local/go/bin:$PATH

RUN curl -L "https://storage.googleapis.com/golang/go${go_version}.linux-amd64.tar.gz" | tar -C /usr/local -xz

ENTRYPOINT ["go", "test"]
