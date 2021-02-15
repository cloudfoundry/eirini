module code.cloudfoundry.org/eirini

go 1.15

replace k8s.io/client-go => k8s.io/client-go v0.20.3

require (
	cloud.google.com/go v0.77.0 // indirect
	code.cloudfoundry.org/bbs v0.0.0-20201203185820-4070ad0e44b1 // indirect
	code.cloudfoundry.org/cfhttp/v2 v2.0.0
	code.cloudfoundry.org/clock v1.0.0 // indirect
	code.cloudfoundry.org/consuladapter v0.0.0-20200131002136-ac1daf48ba97 // indirect
	code.cloudfoundry.org/diego-logging-client v0.0.0-20201207211221-6526582b708b // indirect
	code.cloudfoundry.org/executor v0.0.0-20201214152003-d98dd1d962d6 // indirect
	code.cloudfoundry.org/garden v0.0.0-20210208153517-580cadd489d2 // indirect
	code.cloudfoundry.org/go-diodes v0.0.0-20190809170250-f77fb823c7ee // indirect
	code.cloudfoundry.org/go-loggregator v7.4.0+incompatible
	code.cloudfoundry.org/lager v2.0.0+incompatible
	code.cloudfoundry.org/locket v0.0.0-20210126204241-74d8e4fe8d79 // indirect
	code.cloudfoundry.org/rep v0.0.0-20201204180529-306b13dbb1cc // indirect
	code.cloudfoundry.org/rfc5424 v0.0.0-20201103192249-000122071b78 // indirect
	code.cloudfoundry.org/runtimeschema v0.0.0-20180622184205-c38d8be9f68c
	code.cloudfoundry.org/tlsconfig v0.0.0-20200131000646-bbe0f8da39b3
	code.cloudfoundry.org/tps v0.0.0-20190724214151-ce1ef3913d8e
	code.cloudfoundry.org/urljoiner v0.0.0-20170223060717-5cabba6c0a50 // indirect
	github.com/Azure/go-autorest/autorest v0.11.18 // indirect
	github.com/Microsoft/go-winio v0.4.16 // indirect
	github.com/cockroachdb/apd v1.1.0 // indirect
	github.com/containers/image v3.0.2+incompatible
	github.com/containers/storage v1.25.0 // indirect
	github.com/deckarep/golang-set v1.7.1
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v20.10.3+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.6.3 // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-metrics v0.0.1 // indirect
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7 // indirect
	github.com/go-logr/logr v0.4.0
	github.com/go-sql-driver/mysql v1.5.0 // indirect
	github.com/go-test/deep v1.0.7 // indirect
	github.com/gofrs/flock v0.8.0
	github.com/gofrs/uuid v4.0.0+incompatible // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.2.0 // indirect
	github.com/googleapis/gnostic v0.5.4 // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-multierror v1.1.0
	github.com/hashicorp/go-retryablehttp v0.6.8
	github.com/hashicorp/go-uuid v1.0.2
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/jackc/fake v0.0.0-20150926172116-812a484cc733 // indirect
	github.com/jackc/pgx v3.6.2+incompatible // indirect
	github.com/jessevdk/go-flags v1.4.0
	github.com/jinzhu/copier v0.2.4
	github.com/julienschmidt/httprouter v1.3.0
	github.com/lib/pq v1.9.0 // indirect
	github.com/magefile/mage v1.11.0 // indirect
	github.com/maxbrunsfeld/counterfeiter/v6 v6.3.0
	github.com/mitchellh/mapstructure v1.3.3 // indirect
	github.com/nats-io/jwt v1.2.2 // indirect
	github.com/nats-io/nats-server/v2 v2.1.9
	github.com/nats-io/nats.go v1.10.0
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d // indirect
	github.com/onsi/ginkgo v1.15.0
	github.com/onsi/gomega v1.10.5
	github.com/opencontainers/image-spec v1.0.1
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.9.0
	github.com/prometheus/common v0.16.0
	github.com/prometheus/procfs v0.6.0 // indirect
	github.com/shopspring/decimal v1.2.0 // indirect
	github.com/sirupsen/logrus v1.8.0 // indirect
	github.com/spf13/cobra v1.1.3
	github.com/tedsuo/ifrit v0.0.0-20191009134036-9a97d0632f00 // indirect
	golang.org/x/crypto v0.0.0-20210218145215-b8e89b74b9df // indirect
	golang.org/x/oauth2 v0.0.0-20210216194517-16ff1888fd2e // indirect
	golang.org/x/sys v0.0.0-20210218145245-beda7e5e158e // indirect
	golang.org/x/term v0.0.0-20201210144234-2321bbc49cbf // indirect
	golang.org/x/time v0.0.0-20201208040808-7e3f01d25324 // indirect
	gomodules.xyz/jsonpatch/v2 v2.1.0
	google.golang.org/genproto v0.0.0-20210218151259-fe80b386bf06 // indirect
	google.golang.org/grpc v1.35.0
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
	honnef.co/go/tools v0.0.1-2020.1.5 // indirect
	k8s.io/api v0.20.3
	k8s.io/apiextensions-apiserver v0.20.3 // indirect
	k8s.io/apimachinery v0.20.3
	k8s.io/client-go v1.5.1
	k8s.io/code-generator v0.20.3
	k8s.io/klog v1.0.0
	k8s.io/klog/v2 v2.5.0 // indirect
	k8s.io/kube-openapi v0.0.0-20210216185858-15cd8face8d6 // indirect
	k8s.io/metrics v0.20.3
	sigs.k8s.io/controller-runtime v0.8.2
	sigs.k8s.io/structured-merge-diff/v4 v4.0.3 // indirect
)
