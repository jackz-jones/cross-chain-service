#!/bin/bash

project_name=cross-chain-service
image_name=cross-chain-service
commit_id=$(git rev-parse --short=7 HEAD)
VERSION=v1.1.0

if [[ $(pwd) == *"docker"* ]]; then
  cd ..
fi

docker stop ${image_name}
docker rm  ${image_name}
docker images | grep  ${image_name} | awk '{print $3}' | xargs docker rmi

make build-docker

docker run --log-opt max-size=100m --log-opt max-file=2 \
  -p 6064:6064 -p 8084:8084 --name ${image_name} \
  -v ~/deploy/scripts:/${project_name}/cert \
  -v ~/ida-projects/${project_name}/logs:/${project_name}/logs \
  -d 192.168.1.2:5000/${image_name}-${commit_id}:${VERSION}

docker ps
