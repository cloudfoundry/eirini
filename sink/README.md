a sync loop (similar to nsync) that uses pluggable 'OPI' backends to convert CC apps in to backend resources.

# Usage

~~~~
# make sure ~/.kube/config points to a valid kube
cube sync --ccApi <api.your-env.com> --ccPass <your_internal_user_admin_pass>

# (in another window..) watch your cf apps appear in kube
kubectl get deployments --watch

# push a docker image based app, it'll appear as a deployment in kube next time
the sync loop runs
cf push -o myimage mydockerapp

# nb: this will currently just push busybox because it doesn't now how to grab
# a URI from the registry, but that's easy enough to fix
cf push mybuildpackapp
~~~~

# Todo

Lots! There's tons of missing stuff here, but hopefully it shows the idea. 

 - Env vars, volumes, routes, security groups
 - Updating and deleting apps
 - Responding to CC requests so we dont have to wait for the sync loop and so `cf push/start` can see when the thing has succeeded
 - Caching the list of existing k8s resources and using a watch to keep it updated
 - Moving much of this in to CC (but then you have to write ruby: maybe the opi.Desire interface should be exposed as swagger and the sync loop - which is brain-dead simple - can happen in CC and use that api)
