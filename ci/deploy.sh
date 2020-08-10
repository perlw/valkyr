#!/bin/bash
if [ "$(docker network ls -qf "name=valkyr")" == ""]; then
  docker network create valkyr;
fi;
echo "name $APP_NAME"
#dockerid=$(ssh -o StrictHostKeyChecking=no -i ./key \
#$SSH_HOST \ "docker ps -qf \"ancestor=$APP_NAME\"")
#ssh -o StrictHostKeyChecking=no -i ./key $SSH_HOST \
#'gunzip | docker load' < image.tar.gz
#docker stop $dockerid
#docker run --rm -d -p 8001:80 -p 8444:443 valkyr:latest
#docker container prune -f
#docker image prune -af
