#!/bin/bash
if [ "$(docker network ls -qf \"name=valkyr\")" == ""]; then
  docker network create valkyr;
fi;
dockerid=$(docker ps -qf "ancestor=valkyr")
gunzip -c images/valkyr.tar.gz | docker load
if [ "$dockerid" != ""]; then
  docker stop $dockerid
fi;
docker run --restart unless-stopped -d \
	-p 8000:80 -p 8443:443 --net=valkyr \
	valkyr:latest
docker container prune -f
docker image prune -af
