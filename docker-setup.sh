#!/bin/bash
docker-compose exec db sh -c 'exec mysql -u root -pchangeme writefreely < /tmp/schema.sql'
docker exec writefreely_web_1 writefreely --gen-keys
docker exec -it writefreely_web_1 writefreely --config