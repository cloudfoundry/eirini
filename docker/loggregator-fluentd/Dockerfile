FROM fluent/fluentd-kubernetes-daemonset:v1.4-debian-elasticsearch
ARG GIT_SHA
RUN if [ -z "$GIT_SHA" ]; then echo "GIT_SHA not set"; exit 1; else : ; fi

RUN gem install grpc:1.20.0

COPY ./fluentd/lib/envelope_pb.rb /usr/local/lib/ruby/site_ruby/
COPY ./fluentd/lib/ingress_pb.rb /usr/local/lib/ruby/site_ruby/
COPY ./fluentd/lib/ingress_services_pb.rb /usr/local/lib/ruby/site_ruby/

COPY ./fluentd/plugins/loggregator.rb /fluentd/plugins/out_loggregator.rb
LABEL org.opencontainers.image.revision=$GIT_SHA \
      org.opencontainers.image.source=https://code.cloudfoundry.org/eirini
