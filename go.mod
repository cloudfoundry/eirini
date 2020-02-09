module code.cloudfoundry.org/eirini

go 1.13

replace (
	github.com/googleapis/gnostic => github.com/googleapis/gnostic v0.1.0
	k8s.io/client-go => k8s.io/client-go v0.17.0
)

require (
	cloud.google.com/go v0.52.0 // indirect
	code.cloudfoundry.org/bbs v0.0.0-20200131002608-d0518a556b8f
	code.cloudfoundry.org/cfhttp/v2 v2.0.0
	code.cloudfoundry.org/clock v0.0.0-20180518195852-02e53af36e6c // indirect
	code.cloudfoundry.org/consuladapter v0.0.0-20190222031846-a0ec466a22b6 // indirect
	code.cloudfoundry.org/diego-logging-client v0.0.0-20190918155030-cd01d2d2c629 // indirect
	code.cloudfoundry.org/executor v0.0.0-20191210222949-67a08c48e56c // indirect
	code.cloudfoundry.org/garden v0.0.0-20191128141255-60b076cc4749 // indirect
	code.cloudfoundry.org/go-diodes v0.0.0-20190809170250-f77fb823c7ee // indirect
	code.cloudfoundry.org/go-loggregator v7.4.0+incompatible
	code.cloudfoundry.org/lager v2.0.0+incompatible
	code.cloudfoundry.org/locket v0.0.0-20191127212858-571765e813ca // indirect
	code.cloudfoundry.org/rep v0.0.0-20191210190026-b68fa6668abc // indirect
	code.cloudfoundry.org/rfc5424 v0.0.0-20180905210152-236a6d29298a // indirect
	code.cloudfoundry.org/runtimeschema v0.0.0-20180622184205-c38d8be9f68c
	code.cloudfoundry.org/tlsconfig v0.0.0-20200131000646-bbe0f8da39b3
	code.cloudfoundry.org/tps v0.0.0-20190724214151-ce1ef3913d8e
	code.cloudfoundry.org/urljoiner v0.0.0-20170223060717-5cabba6c0a50 // indirect
	github.com/Azure/go-autorest/autorest v0.9.5 // indirect
	github.com/deckarep/golang-set v1.7.1
	github.com/go-sql-driver/mysql v1.4.1 // indirect
	github.com/go-test/deep v1.0.4 // indirect
	github.com/gogo/protobuf v1.3.1
	github.com/google/gofuzz v1.1.0 // indirect
	github.com/googleapis/gnostic v0.4.1 // indirect
	github.com/gophercloud/gophercloud v0.7.0 // indirect
	github.com/hashicorp/consul/api v1.3.0 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/imdario/mergo v0.3.8 // indirect
	github.com/jessevdk/go-flags v1.4.0
	github.com/json-iterator/go v1.1.9 // indirect
	github.com/julienschmidt/httprouter v1.3.0
	github.com/lib/pq v1.2.0 // indirect
	github.com/nats-io/nats-server/v2 v2.1.2
	github.com/nats-io/nats.go v1.9.1
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d // indirect
	github.com/onsi/ginkgo v1.12.0
	github.com/onsi/gomega v1.7.1
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v0.0.5
	github.com/tedsuo/ifrit v0.0.0-20191009134036-9a97d0632f00 // indirect
	go.uber.org/atomic v1.5.1 // indirect
	go.uber.org/multierr v1.4.0
	golang.org/x/crypto v0.0.0-20200208060501-ecb85df21340 // indirect
	golang.org/x/lint v0.0.0-20200130185559-910be7a94367 // indirect
	golang.org/x/net v0.0.0-20200202094626-16171245cfb2 // indirect
	golang.org/x/sys v0.0.0-20200202164722-d101bd2416d5 // indirect
	golang.org/x/time v0.0.0-20191024005414-555d28b269f0 // indirect
	golang.org/x/tools v0.0.0-20200207224406-61798d64f025 // indirect
	google.golang.org/genproto v0.0.0-20200207204624-4f3edf09f4f6 // indirect
	google.golang.org/grpc v1.27.1 // indirect
	gopkg.in/yaml.v2 v2.2.8
	k8s.io/api v0.17.2
	k8s.io/apimachinery v0.17.2
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/klog v1.0.0
	k8s.io/metrics v0.17.2
	k8s.io/utils v0.0.0-20200124190032-861946025e34 // indirect
	sigs.k8s.io/yaml v1.2.0 // indirect
)
