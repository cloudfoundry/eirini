module code.cloudfoundry.org/eirini

go 1.17

replace (
	github.com/go-logr/logr => github.com/go-logr/logr v0.4.0
	github.com/googleapis/gnostic => github.com/googleapis/gnostic v0.5.5
	k8s.io/klog/v2 => k8s.io/klog/v2 v2.10.0
)

require (
	cloud.google.com/go v0.99.0 // indirect
	code.cloudfoundry.org/bbs v0.0.0-20211111162920-83003cff4ff1 // indirect
	code.cloudfoundry.org/cfhttp/v2 v2.0.0
	code.cloudfoundry.org/lager v2.0.0+incompatible
	code.cloudfoundry.org/runtimeschema v0.0.0-20210817192503-36a2cb16a206
	code.cloudfoundry.org/tlsconfig v0.0.0-20211123175040-23cc9f05b6b3
	code.cloudfoundry.org/tps v0.0.0-20210909223736-1b7791ea46f7
	github.com/Azure/go-autorest/autorest v0.11.23 // indirect
	github.com/Azure/go-autorest/autorest/adal v0.9.17 // indirect
	github.com/BurntSushi/toml v0.4.1 // indirect
	github.com/containers/image v3.0.2+incompatible
	github.com/containers/storage v1.32.1 // indirect
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v20.10.11+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.6.4 // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7 // indirect
	github.com/fatih/color v1.12.0 // indirect
	github.com/go-logr/logr v1.2.1
	github.com/gofrs/flock v0.8.1
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hashicorp/go-retryablehttp v0.7.0
	github.com/hashicorp/go-uuid v1.0.2
	github.com/jessevdk/go-flags v1.5.0
	github.com/julienschmidt/httprouter v1.3.0
	github.com/maxbrunsfeld/counterfeiter/v6 v6.4.1
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.17.0
	github.com/opencontainers/image-spec v1.0.2
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.11.0
	github.com/prometheus/common v0.32.1 // indirect
	github.com/prometheus/procfs v0.7.3 // indirect
	golang.org/x/crypto v0.0.0-20211209193657-4570a0811e8b // indirect
	golang.org/x/net v0.0.0-20211209124913-491a49abca63 // indirect
	golang.org/x/oauth2 v0.0.0-20211104180415-d3ed0bb246c8 // indirect
	golang.org/x/sys v0.0.0-20211210111614-af8b64212486 // indirect
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211 // indirect
	golang.org/x/text v0.3.7 // indirect
	gomodules.xyz/jsonpatch/v2 v2.2.0
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.23.0
	k8s.io/apiextensions-apiserver v0.23.0 // indirect
	k8s.io/apimachinery v0.23.0
	k8s.io/client-go v0.23.0
	k8s.io/klog/v2 v2.30.0
	k8s.io/kube-openapi v0.0.0-20211115234752-e816edb12b65 // indirect
	k8s.io/utils v0.0.0-20211208161948-7d6a63dca704 // indirect
	sigs.k8s.io/controller-runtime v0.10.3
)

require (
	code.cloudfoundry.org/urljoiner v0.0.0-20170223060717-5cabba6c0a50 // indirect
	github.com/Azure/go-autorest v14.2.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest/date v0.3.0 // indirect
	github.com/Azure/go-autorest/logger v0.2.1 // indirect
	github.com/Azure/go-autorest/tracing v0.6.0 // indirect
	github.com/Microsoft/go-winio v0.5.1 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/docker/go-metrics v0.0.1 // indirect
	github.com/evanphx/json-patch v5.6.0+incompatible // indirect
	github.com/fsnotify/fsnotify v1.5.1 // indirect
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt/v4 v4.2.0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/go-cmp v0.5.6 // indirect
	github.com/googleapis/gnostic v0.5.7 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mattn/go-colorable v0.1.8 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.2-0.20181231171920-c182affec369 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/nxadm/tail v1.4.8 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/sirupsen/logrus v1.8.1 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/square/certstrap v1.2.0 // indirect
	golang.org/x/mod v0.5.1 // indirect
	golang.org/x/time v0.0.0-20211116232009-f0f3c7e86c11 // indirect
	golang.org/x/tools v0.1.8 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.27.1 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
	k8s.io/component-base v0.23.0 // indirect
	sigs.k8s.io/json v0.0.0-20211208200746-9f7c6b3444d2 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.0 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)
