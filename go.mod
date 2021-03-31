module code.cloudfoundry.org/eirini

go 1.16

replace k8s.io/client-go => k8s.io/client-go v0.20.3

require (
	cloud.google.com/go v0.80.0 // indirect
	code.cloudfoundry.org/cfhttp/v2 v2.0.0
	code.cloudfoundry.org/go-loggregator v7.4.0+incompatible
	code.cloudfoundry.org/lager v2.0.0+incompatible
	code.cloudfoundry.org/runtimeschema v0.0.0-20180622184205-c38d8be9f68c
	code.cloudfoundry.org/tlsconfig v0.0.0-20200131000646-bbe0f8da39b3
	github.com/Azure/go-autorest/autorest v0.11.18 // indirect
	github.com/Microsoft/go-winio v0.4.16 // indirect
	github.com/cloudfoundry/tps v0.0.0-20210303221408-6295d5712990
	github.com/containers/image v3.0.2+incompatible
	github.com/containers/storage v1.25.0 // indirect
	github.com/deckarep/golang-set v1.7.1
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v20.10.5+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.6.3 // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-metrics v0.0.1 // indirect
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7 // indirect
	github.com/go-logr/logr v0.4.0
	github.com/gofrs/flock v0.8.0
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.2.0 // indirect
	github.com/googleapis/gnostic v0.5.4 // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hashicorp/go-retryablehttp v0.6.8
	github.com/hashicorp/go-uuid v1.0.2
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/jessevdk/go-flags v1.5.0
	github.com/jinzhu/copier v0.2.8
	github.com/julienschmidt/httprouter v1.3.0
	github.com/klauspost/compress v1.11.13 // indirect
	github.com/maxbrunsfeld/counterfeiter/v6 v6.3.0
	github.com/minio/highwayhash v1.0.2 // indirect
	github.com/mitchellh/mapstructure v1.3.3 // indirect
	github.com/nats-io/nats-server/v2 v2.2.0
	github.com/nats-io/nats.go v1.10.1-0.20210228004050-ed743748acac
	github.com/onsi/ginkgo v1.15.2
	github.com/onsi/gomega v1.11.0
	github.com/opencontainers/image-spec v1.0.1
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.10.0
	github.com/prometheus/common v0.20.0
	github.com/sirupsen/logrus v1.8.1 // indirect
	github.com/spf13/cobra v1.1.3
	golang.org/x/crypto v0.0.0-20210322153248-0c34fe9e7dc2 // indirect
	golang.org/x/mod v0.4.2 // indirect
	golang.org/x/net v0.0.0-20210330230544-e57232859fb2 // indirect
	golang.org/x/oauth2 v0.0.0-20210323180902-22b0adad7558 // indirect
	golang.org/x/term v0.0.0-20210317153231-de623e64d2a6 // indirect
	golang.org/x/time v0.0.0-20210220033141-f8bda1e9f3ba // indirect
	gomodules.xyz/jsonpatch/v2 v2.1.0
	google.golang.org/genproto v0.0.0-20210330181207-2295ebbda0c6 // indirect
	google.golang.org/grpc v1.36.1
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
	k8s.io/api v0.20.5
	k8s.io/apiextensions-apiserver v0.20.5 // indirect
	k8s.io/apimachinery v0.20.5
	k8s.io/client-go v1.5.2
	k8s.io/code-generator v0.20.5
	k8s.io/klog v1.0.0
	k8s.io/klog/v2 v2.8.0 // indirect
	k8s.io/kube-openapi v0.0.0-20210323165736-1a6458611d18 // indirect
	k8s.io/metrics v0.20.5
	k8s.io/utils v0.0.0-20210305010621-2afb4311ab10 // indirect
	sigs.k8s.io/controller-runtime v0.8.3
	sigs.k8s.io/structured-merge-diff/v4 v4.1.0 // indirect
)
