#!/bin/bash
if [ "$(docker network ls -qf \"name=valkyr\")" == ""]; then
  docker network create valkyr;
fi;
dockerid=$(docker ps -qf "ancestor=valkyr")
gunzip -c images/valkyr.tar.gz | docker load
docker stop $dockerid
docker run --rm -d -p 8003:80 -p 8444:443 valkyr:latest
docker container prune -f
docker image prune -af
