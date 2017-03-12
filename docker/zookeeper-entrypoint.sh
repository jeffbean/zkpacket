#!/bin/bash

echo "${MYID:-1}" > /tmp/zookeeper/myid


cat > /opt/zookeeper/conf/zoo.cfg <<EOF
tickTime=2000
initLimit=10
syncLimit=5
dataDir=/tmp/zookeeper
standaloneEnabled=false
clientPort=2181
dynamicConfigFile=/opt/zookeeper/conf/zoo.cfg.dynamic
EOF

curl --unix-socket /var/run/docker.sock http:/containers/json?filters='{"ancestor":["jplock/zookeeper:3.5.1-alpha"]}'


exec "$@"