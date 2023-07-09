#!/bin/bash
dockerid=$(docker ps -qf "ancestor=valkyr")
gunzip -c images/valkyr.tar.gz | docker load
if [ "$dockerid" != "" ]; then
  docker stop $dockerid
fi;
docker run --restart unless-stopped -d --net=host \
	-v /etc/valkyr.json:/etc/valkyr.json \
	valkyr:latest
docker container prune -f
docker image prune -af
