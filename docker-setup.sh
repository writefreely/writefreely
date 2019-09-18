#! /bin/bash
# this script will configure and intialize the persitent data needed
# for a writefreely instance using the docker-compose.yml in this repo

# start database
docker run --rm -d --volume=writefreely_db-data:/var/lib/mysql \
--name=db \
-e "MYSQL_DATABASE=writefreely" \
-e "MYSQL_ROOT_PASSWORD=changeme" \
-p 3306:3306 \
mariadb:latest 

# create new asset signing keys
docker run --rm --volume=writefreely_web-data:/home/writefreely writeas/writefreely:latest "-gen-keys"

# generate new configuration and initialize database
docker run --rm -it --volume=writefreely_web-data:/home/writefreely \
--link db:db \
writeas/writefreely:latest "-config"

# clean up detached database container
docker container stop db