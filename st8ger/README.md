# St8ger

The st8ger implements Staging by running Kubernetes/OPI one-off tasks. It receives staging requests from the CloudController, translates them to an OPI (staging) task, and schedules the task on Kubernetes. The code that is run by the job container on Kubernetes is located in `cube/recipe`. It is basically responsible to download app-bits, staging, and uploading the resulting droplet (read more about the recipe job in `cube/recipe/README.md`). 

## Testing the St8ger on a Lite Environment

1. Get a Bosh-Lite director 

2. Deploy the latest CF with the [cube-release](https://github.com/andrew-edgar/cube-release) to your bosh-lite (follow the instructions in the README).

3. Get a Minikube deployment and make sure Bosh-Lite and Minikube are running in the same Network:

  When starting `minikube` you can define a CIDR. You need to make sure that it is in the same subnet as your bosh-lite installation. By default this should be `192.168.50.1/24`. 

  `$ minikube start --host-only-cidr=192.186.50.1/24`

  This is important as Cube and Kube need to communicate in both directions.


4. Push an app to your CF. 


## Try the stager locally

If you have an running bosh-lite and a minikube you can run the `st8ger` locally and create a fake staging request by providing a app guid of an existing app.

Run the stager like this:

```
cube stage \
  --kubeconfig ~/.kube/config \
  --cf-username admin \
  --cf-password <your-password> \ 
  --cf-endpoint https://api.bosh-lite.com
```

