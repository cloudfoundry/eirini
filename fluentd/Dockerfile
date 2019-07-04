FROM fluent/fluentd-kubernetes-daemonset:v1.4-debian-elasticsearch

RUN gem install grpc:1.20.0

COPY ./lib/envelope_pb.rb /usr/local/lib/ruby/site_ruby/
COPY ./lib/ingress_pb.rb /usr/local/lib/ruby/site_ruby/
COPY ./lib/ingress_services_pb.rb /usr/local/lib/ruby/site_ruby/

COPY ./plugins/loggregator.rb /fluentd/plugins/out_loggregator.rb
