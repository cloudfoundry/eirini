module code.cloudfoundry.org/eirini

go 1.13

replace (
	github.com/coreos/bbolt => go.etcd.io/bbolt v1.3.2
	github.com/go-logr/logr => github.com/go-logr/logr v0.1.0
	github.com/googleapis/gnostic => github.com/googleapis/gnostic v0.4.0
	google.golang.org/grpc => google.golang.org/grpc v1.29.1
	k8s.io/api => k8s.io/api v0.18.8
	k8s.io/apimachinery => k8s.io/apimachinery v0.18.8
	k8s.io/client-go => k8s.io/client-go v0.18.6
	k8s.io/klog/v2 => k8s.io/klog/v2 v2.1.0
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20200204173128-addea2498afe
)

require (
	cloud.google.com/go v0.65.0 // indirect
	code.cloudfoundry.org/bbs v0.0.0-20200615191359-7b6fa295fa8d // indirect
	code.cloudfoundry.org/cfhttp/v2 v2.0.0
	code.cloudfoundry.org/clock v0.0.0-20180518195852-02e53af36e6c // indirect
	code.cloudfoundry.org/consuladapter v0.0.0-20190222031846-a0ec466a22b6 // indirect
	code.cloudfoundry.org/diego-logging-client v0.0.0-20190918155030-cd01d2d2c629 // indirect
	code.cloudfoundry.org/eirinix v0.3.1-0.20200813115927-6a0925613552
	code.cloudfoundry.org/executor v0.0.0-20191210222949-67a08c48e56c // indirect
	code.cloudfoundry.org/garden v0.0.0-20191128141255-60b076cc4749 // indirect
	code.cloudfoundry.org/go-diodes v0.0.0-20190809170250-f77fb823c7ee // indirect
	code.cloudfoundry.org/go-loggregator v7.4.0+incompatible
	code.cloudfoundry.org/lager v2.0.0+incompatible
	code.cloudfoundry.org/locket v0.0.0-20191127212858-571765e813ca // indirect
	code.cloudfoundry.org/quarks-utils v0.0.0-20200901084422-8ffd423001d8 // indirect
	code.cloudfoundry.org/rep v0.0.0-20191210190026-b68fa6668abc // indirect
	code.cloudfoundry.org/rfc5424 v0.0.0-20180905210152-236a6d29298a // indirect
	code.cloudfoundry.org/runtimeschema v0.0.0-20180622184205-c38d8be9f68c
	code.cloudfoundry.org/tlsconfig v0.0.0-20200131000646-bbe0f8da39b3
	code.cloudfoundry.org/tps v0.0.0-20190724214151-ce1ef3913d8e
	code.cloudfoundry.org/urljoiner v0.0.0-20170223060717-5cabba6c0a50 // indirect
	github.com/Azure/go-autorest/autorest v0.11.4 // indirect
	github.com/Azure/go-autorest/autorest/adal v0.9.2 // indirect
	github.com/Djarvur/go-err113 v0.1.0 // indirect
	github.com/cockroachdb/apd v1.1.0 // indirect
	github.com/containers/image v3.0.2+incompatible
	github.com/containers/storage v1.16.0 // indirect
	github.com/coreos/bbolt v1.3.5 // indirect
	github.com/coreos/etcd v3.3.25+incompatible // indirect
	github.com/coreos/go-systemd v0.0.0-20191104093116-d3cd4ed1dbcf // indirect
	github.com/daixiang0/gci v0.2.4 // indirect
	github.com/deckarep/golang-set v1.7.1
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v1.13.1 // indirect
	github.com/docker/docker-credential-helpers v0.6.3 // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-metrics v0.0.1 // indirect
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7 // indirect
	github.com/go-critic/go-critic v0.5.2 // indirect
	github.com/go-logr/logr v0.2.0
	github.com/go-test/deep v1.0.5 // indirect
	github.com/gofrs/flock v0.8.0 // indirect
	github.com/gofrs/uuid v3.3.0+incompatible // indirect
	github.com/golangci/golangci-lint v1.30.0 // indirect
	github.com/golangci/misspell v0.3.5 // indirect
	github.com/google/certificate-transparency-go v1.1.0 // indirect
	github.com/google/go-cmp v0.5.2 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/monologue v0.0.0-20200310112848-e585696c5f1b // indirect
	github.com/google/uuid v1.1.2 // indirect
	github.com/googleapis/gnostic v0.5.1 // indirect
	github.com/gophercloud/gophercloud v0.12.0 // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/gostaticanalysis/analysisutil v0.1.0 // indirect
	github.com/gostaticanalysis/comment v1.4.0 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.1 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.14.7 // indirect
	github.com/hashicorp/go-multierror v1.1.0
	github.com/hashicorp/go-retryablehttp v0.6.7
	github.com/hashicorp/go-uuid v1.0.2
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/jackc/fake v0.0.0-20150926172116-812a484cc733 // indirect
	github.com/jackc/pgx v3.6.2+incompatible // indirect
	github.com/jessevdk/go-flags v1.4.0
	github.com/jinzhu/copier v0.0.0-20190924061706-b57f9002281a
	github.com/jirfag/go-printf-func-name v0.0.0-20200119135958-7558a9eaa5af // indirect
	github.com/jonboulle/clockwork v0.2.0 // indirect
	github.com/julienschmidt/httprouter v1.3.0
	github.com/magiconair/properties v1.8.2 // indirect
	github.com/matoous/godox v0.0.0-20200801072554-4fb83dc2941e // indirect
	github.com/mattn/go-runewidth v0.0.9 // indirect
	github.com/maxbrunsfeld/counterfeiter/v6 v6.2.3
	github.com/mitchellh/mapstructure v1.3.3 // indirect
	github.com/nats-io/jwt v1.0.1 // indirect
	github.com/nats-io/nats-server/v2 v2.1.7
	github.com/nats-io/nats.go v1.10.0
	github.com/nats-io/nkeys v0.2.0 // indirect
	github.com/nishanths/exhaustive v0.0.0-20200811152831-6cf413ae40e0 // indirect
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d // indirect
	github.com/olekukonko/tablewriter v0.0.4 // indirect
	github.com/onsi/ginkgo v1.14.0
	github.com/onsi/gomega v1.10.1
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.0.1
	github.com/pelletier/go-toml v1.8.0 // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus/common v0.13.0 // indirect
	github.com/quasilyte/regex/syntax v0.0.0-20200805063351-8f842688393c // indirect
	github.com/shopspring/decimal v1.2.0 // indirect
	github.com/spf13/afero v1.3.4 // indirect
	github.com/spf13/cast v1.3.1 // indirect
	github.com/spf13/cobra v1.0.0
	github.com/spf13/viper v1.7.1 // indirect
	github.com/ssgreg/nlreturn/v2 v2.1.0 // indirect
	github.com/stretchr/objx v0.3.0 // indirect
	github.com/tdakkota/asciicheck v0.0.0-20200416200610-e657995f937b // indirect
	github.com/tedsuo/ifrit v0.0.0-20191009134036-9a97d0632f00 // indirect
	github.com/timakin/bodyclose v0.0.0-20200424151742-cb6215831a94 // indirect
	github.com/tmc/grpc-websocket-proxy v0.0.0-20200427203606-3cfed13b9966 // indirect
	github.com/ultraware/funlen v0.0.3 // indirect
	github.com/urfave/cli v1.22.4 // indirect
	github.com/weppos/publicsuffix-go v0.13.0 // indirect
	github.com/zmap/zcrypto v0.0.0-20200731193947-bf025252fa41 // indirect
	github.com/zmap/zlint v1.1.0 // indirect
	go.etcd.io/etcd v3.3.25+incompatible // indirect
	go.uber.org/multierr v1.5.0
	go.uber.org/zap v1.16.0
	golang.org/x/crypto v0.0.0-20200820211705-5c72a883971a // indirect
	golang.org/x/sys v0.0.0-20200831180312-196b9ba8737a // indirect
	golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e // indirect
	golang.org/x/tools v0.0.0-20200902012652-d1954cc86c82 // indirect
	gomodules.xyz/jsonpatch/v2 v2.1.0
	google.golang.org/genproto v0.0.0-20200901141002-b3bf27a9dbd1 // indirect
	google.golang.org/grpc v1.31.1
	gopkg.in/ini.v1 v1.60.2 // indirect
	gopkg.in/yaml.v2 v2.3.0
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776 // indirect
	honnef.co/go/tools v0.0.1-2020.1.5 // indirect
	k8s.io/api v0.19.0
	k8s.io/apiextensions-apiserver v0.19.0 // indirect
	k8s.io/apimachinery v0.19.0
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/code-generator v0.19.0
	k8s.io/klog v1.0.0
	k8s.io/klog/v2 v2.3.0 // indirect
	k8s.io/kube-openapi v0.0.0-20200831175022-64514a1d5d59 // indirect
	k8s.io/metrics v0.19.0
	k8s.io/utils v0.0.0-20200821003339-5e75c0163111 // indirect
	mvdan.cc/gofumpt v0.0.0-20200802201014-ab5a8192947d // indirect
	mvdan.cc/unparam v0.0.0-20200501210554-b37ab49443f7 // indirect
	sigs.k8s.io/controller-runtime v0.6.2
)
