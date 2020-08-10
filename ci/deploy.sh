#!/bin/bash
if [ "$(docker network ls -qf \"name=valkyr\")" == ""]; then
  docker network create valkyr;
fi;
gunzip -c images/valkyr.tar.gz | docker load
dockerid=$(docker ps -qf "ancestor=valkyr")
docker stop $dockerid
docker run --rm -d -p 8001:80 -p 8444:443 valkyr:latest
docker container prune -f
docker image prune -af
