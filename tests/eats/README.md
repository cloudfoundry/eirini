# Eirini Acceptance Tests (EATs)

## How to run tests against various environments

### Local Kind

Firstly, target kind: `kind export kubeconfig --name ENV-NAME`

#### Using the Eirini deployment files (aka Helmless)

```
$EIRINI_RELEASE_DIR/deploy/scripts/cleanup.sh
$EIRINI_RELEASE_DIR/deploy/scripts/deploy.sh

EIRINI_ADDRESS=https://$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[0].address}') \
EIRINI_TLS_SECRET=eirini-certs \
EIRINI_SYSTEM_NS=eirini-core \
$EIRINI_DIR/scripts/run_eats_tests.sh
```

### cf-for-k8s

1. Target an environment
1. Patch-me-if-you-can for the modified components
1. Remove the eirini network policy
1. Make the local EATs Fixture test code ignore certificate validation by setting InsecureSkipVerify: true in the TLSConfig
1. Add an Istio virtual service (substituting ENV-NAME as appropriate):
   ```
   apiVersion: networking.istio.io/v1beta1
   kind: VirtualService
   metadata:
     name: eirini-external
     namespace: cf-system
   spec:
     gateways:
     - cf-system/istio-ingressgateway
     hosts:
     - eirini.ENV-NAME.ci-envs.eirini.cf-app.com
     http:
     - route:
       - destination:
           host: eirini.cf-system.svc.cluster.local
           port:
             number: 8080
   ```
1. ```
   EIRINI_ADDRESS=https://eirini.ENV-NAME.ci-envs.eirini.cf-app.com \
   EIRINI_TLS_SECRET=eirini-internal-tls-certs-ver-1 \
   EIRINI_SYSTEM_NS=cf-system \
   ./scripts/run_eats_tests.sh
   ```

Expect a failure due to name of the configmap in cf-for-k8s
