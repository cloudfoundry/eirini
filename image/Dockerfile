FROM ubuntu:latest

COPY opi /eirini/
COPY eirinifs.tar /eirini/
COPY config.yml /eirini/config.yml
COPY kube.conf /eirini/kube.conf

ENTRYPOINT [ "/eirini/opi", \
	"connect", \
	"--config", \
	"/eirini/config.yml" \
]
