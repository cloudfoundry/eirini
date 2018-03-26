# Recipe

`recipe` is a container responsible for downloading app-bits/package from the blobstore, staging, and uploading the resulting droplet back to the blobstore. 

## Build the Container

To build the container you will need to init the `github.com/sclevine/packs` submodule first:

`$ git init submodules --recursive`

Now you can run the build script located in `recipe/bin/`:

`$ bin/build.sh`

The container name is `cube/recipe` and is tagged with `build`.

## Run the container locally with Docker

The container requires several environment variables to run the staging recipe locally:

- `APP_ID`: The app-guid of an cf-app
- `STAGING_GUID`: The staging-guid (it can be an arbitrary value)
- `API_ADDRESS`: The endpoint of your CloudFoundry (eg `https://api.bosh-lite.com`)
- `CF_USERNAME`: The cf admin username
- `CF_PASSWORD`: The cf admin password

These environment variables need to be passed to the docker run call:

`$ docker run -it -e 'APP_ID=<your-app-id>' -e 'STAGING_GUID=staging-guid' -e 'CF_USERNAME=admin' -e 'CF_PASSWORD=<cf-password>' -e 'API_ADDRESS=<cf-endpoint>' --rm cube/recipe:build`

### Info

The docker image is also available on DockerHub: `diegoteam/recipe:build`

If you don't want to build the container you can use the existing one:

`$ docker run -it -e 'APP_ID=<your-app-id>' -e 'STAGING_GUID=staging-guid' -e 'CF_USERNAME=admin' -e 'CF_PASSWORD=<cf-password>' -e 'API_ADDRESS=<cf-endpoint>' --rm diegoteam/recipe:build`

