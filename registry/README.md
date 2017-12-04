# What this?
this a little OCI registry backed by CF droplets. 

# y?

lets you use docker/k8s to schedule staged apps as container images.

# Wattabout patching?

Works the same as today: the registry always combines the droplet layer with the latest rootfs layer, so whenever you pull you get the specific droplet on top of the latest patched rootfs, just as an OCI image rather than having to combine things imperatively.

# Usage

~~~~
# start the registry with the current rootfs
# (a nice way to do this is to deploy `rootfs-release` and `cube-release` on the same bosh host and point one to the other)
cube registry --rootfs /path/to/rootfs.tar

# .. add <yourip>:8080 to insecure-registry-list in your docker daemon

# "stage" the droplet (this is just to convert it to a content-addressed layer, ideally we 
# change staging so it's already stored as a valid layer and then this step goes away)
curl <your ip>:8080/v2/some-space/some-app/blobs/?guid=droplet-guid -XPOST -d@<path-to-droplet-tar-gz> -H"Content-Type: text/plain"

# run your image with docker! push it to kube! push it to cube! world is your oyster
docker run -it -uvcap <your ip>:8080/some-space/some-app:droplet-guid /bin/sh
~~~~

# Other ways?

The alternate way of doing this would be to use an existing registry and just
have a sync loop to pull droplets / rootfs from CC and push them to it. That's
not as expensive as it sounds because you never have to re-upload stuff, blobs
that are there will hit cache in the registry. (Either way the rest of Cube
doesn't really care tho and this is an easier PoC so this is what we did.
Potentially could support both modes like built-in/remote postgres.)

# Implementation notes

Because currently droplet blobs are not stored by guid (i.e. they're not
content-addressed), we have to store a little lookup table so we know the
digest for a particular blob which involves a curl during staging. This
would/should go away by making staging store proper content-addressed
layer-format droplets in the first place.

