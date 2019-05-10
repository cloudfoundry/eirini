FROM fluent/fluentd-kubernetes-daemonset:v0.12-debian-elasticsearch

RUN gem install grpc

COPY ./lib/envelope_pb.rb /usr/local/lib/site_ruby/
COPY ./lib/ingress_pb.rb /usr/local/lib/site_ruby/
COPY ./lib/ingress_services_pb.rb /usr/local/lib/site_ruby/

COPY ./plugins/loggregator.rb /fluentd/plugins/
