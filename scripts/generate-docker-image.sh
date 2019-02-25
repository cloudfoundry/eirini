#!/bin/bash

print_example_config(){
	echo "Please provide a config.yml in <EIRINI_ROOT_DIR>/image. Example: "
	cat << EOF
opi:
  kube_namespace: "the kubernetes namespace used by the cf deployment"
  kube_endpoint: "the kubernetes endpoint where to schedule workload to"
  registry_endpoint: "the eirini registry endpoint (usually the eirini Host maschine on port 8080)"
  api_endpoint: "the CF API endpoint (eg. api.bosh-lite.com)"
  cf_username: "cf admin user"
  cf_password: "cf admin password"
  external_eirini_address: "the external eirini address"
  skip_ssl_validation: "cf CLI skip-ssl-validation flag (bool)"
  insecure_skip_verify: "http client config for insecure traffic (bool)"
EOF
}

verify_exit_code() {
	local exit_code=$1
	local error_msg=$2
	if [ "$exit_code" -ne 0 ]; then
		echo "$error_msg"
		exit 1
	fi
}

#1. create opi
build_opi(){
	GOOS=linux CGO_ENABLED=0 go build -a -o image/opi code.cloudfoundry.org/eirini/cmd/opi
	verify_exit_code $? "Failed to build eirini"
}

#2. create eirinifs.tar
create_eirinifs(){
	./launcher/bin/build-eirinifs.sh && \
	cp launcher/image/eirinifs.tar ./image

	verify_exit_code $? "Failed to create eirinifs.tar"
}
#3. Check if config exists
verify_config_file_exists(){
	if ! [ -f ./image/config.yml ]; then
		print_example_config
		exit 1
	fi
}

#4. Create docker-image
create_docker_image() {
	pushd ./image
	docker build . -t eirini
	verify_exit_code $? "Failed to create docker image"
}

main(){
	echo "Creating Eirini docker image..."
	build_opi
	create_eirinifs
  verify_config_file_exists
	create_docker_image
	echo "Eirini docker image created"
}

main
