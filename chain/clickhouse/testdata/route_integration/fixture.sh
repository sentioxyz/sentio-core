#!/usr/bin/env bash
# Brings up, or down, the topology of TestRouteIntegration (store_route_integration_test.go):
#
#   sfro-c1      compute node holding the Distributed facades, cluster `compute`
#   sfro-d1/d2   one shard of two replicas, cluster `data`, the local slot tables
#   sfro-r1      one replica, cluster `control`, the local range table
#   sfro-keeper  ClickHouse Keeper for all of them, embedded in a server
#
# Every container is on a network of its own and has its own hostName(). sfro-c1 reaches d1, d2 and
# r1 only through proxy.py on the gateway of that network, so the test can hold what the facades
# send them; the replicas reach each other directly. Only the native port of each server is
# published, on 127.0.0.1. Nothing outside the containers, the network and $SFRO_DIR is touched.
#
#   SFRO_IMAGE=<clickhouse-server image> SFRO_DIR=/tmp/sfro fixture.sh up|down
#
# `up` prints the environment the test reads.
set -euo pipefail

cmd=${1:?up or down}
dir=${SFRO_DIR:-/tmp/sfro-route-it}
image=${SFRO_IMAGE:-clickhouse/clickhouse-server:26.3}
net=sfro-net
here=$(cd "$(dirname "$0")" && pwd)
nodes=(c1 d1 d2 r1)
declare -A tcp=([c1]=9000 [d1]=39101 [d2]=39102 [r1]=39103)
declare -A published=([c1]=39100 [d1]=39111 [d2]=39112 [r1]=39113)

down() {
  [[ -f $dir/proxy.pid ]] && kill "$(cat "$dir/proxy.pid")" 2> /dev/null || true
  for n in keeper "${nodes[@]}"; do docker rm -f "sfro-$n" > /dev/null 2>&1 || true; done
  docker network rm "$net" > /dev/null 2>&1 || true
}

if [[ $cmd == down ]]; then
  down
  exit 0
fi

mkdir -p "$dir/state"
rm -f "$dir"/state/hold-*
docker network create "$net" > /dev/null
gw=$(docker network inspect "$net" -f '{{(index .IPAM.Config 0).Gateway}}')

cat > "$dir/keeper.xml" << XML
<clickhouse>
  <listen_host>0.0.0.0</listen_host>
  <keeper_server>
    <tcp_port>9181</tcp_port>
    <server_id>1</server_id>
    <log_storage_path>/tmp/keeper/log</log_storage_path>
    <snapshot_storage_path>/tmp/keeper/snapshots</snapshot_storage_path>
    <raft_configuration><server><id>1</id><hostname>sfro-keeper</hostname><port>9234</port></server></raft_configuration>
  </keeper_server>
</clickhouse>
XML
# The server runs the keeper embedded, not every build has the standalone one.
docker run -d --name sfro-keeper --hostname sfro-keeper --network "$net" -e CLICKHOUSE_SKIP_USER_SETUP=1 \
  -v "$dir/keeper.xml:/etc/clickhouse-server/config.d/keeper.xml:ro" "$image" > /dev/null

for n in "${nodes[@]}"; do
  cat > "$dir/$n.xml" << XML
<clickhouse>
  <listen_host>0.0.0.0</listen_host>
  <tcp_port>${tcp[$n]}</tcp_port>
  <interserver_http_host>sfro-$n</interserver_http_host>
  <logger><level>information</level></logger>
  <zookeeper><node><host>sfro-keeper</host><port>9181</port></node></zookeeper>
  <macros><replica>sfro-$n</replica></macros>
  <remote_servers replace="1">
    <compute><shard><replica><host>sfro-c1</host><port>9000</port></replica></shard></compute>
    <data><shard><internal_replication>true</internal_replication>
      <replica><host>sfro-d1</host><port>39101</port></replica>
      <replica><host>sfro-d2</host><port>39102</port></replica>
    </shard></data>
    <control><shard><internal_replication>true</internal_replication>
      <replica><host>sfro-r1</host><port>39103</port></replica>
    </shard></control>
  </remote_servers>
</clickhouse>
XML
  extra=()
  if [[ $n == c1 ]]; then
    # The facades reach the replicas through the proxy only.
    extra=(--add-host "sfro-d1:$gw" --add-host "sfro-d2:$gw" --add-host "sfro-r1:$gw")
  fi
  docker run -d --name "sfro-$n" --hostname "sfro-$n" --network "$net" "${extra[@]}" \
    -e CLICKHOUSE_SKIP_USER_SETUP=1 -p "127.0.0.1:${published[$n]}:${tcp[$n]}" \
    -v "$dir/$n.xml:/etc/clickhouse-server/config.d/sfro.xml:ro" "$image" > /dev/null
done

targets=()
for n in d1 d2 r1; do
  ip=$(docker inspect "sfro-$n" -f "{{(index .NetworkSettings.Networks \"$net\").IPAddress}}")
  targets+=("${tcp[$n]}=$ip:${tcp[$n]}")
done
nohup python3 -I -B "$here/proxy.py" "$dir/state" "$gw" "${targets[@]}" > "$dir/proxy.log" 2>&1 &
echo $! > "$dir/proxy.pid"

for n in "${nodes[@]}"; do
  for _ in $(seq 120); do
    docker exec "sfro-$n" clickhouse-client --port "${tcp[$n]}" -q 'SELECT 1' > /dev/null 2>&1 && break
    sleep 1
  done
done

echo "SFRO_COMPUTE=127.0.0.1:${published[c1]}"
echo "SFRO_D1=127.0.0.1:${published[d1]}"
echo "SFRO_D2=127.0.0.1:${published[d2]}"
echo "SFRO_R1=127.0.0.1:${published[r1]}"
echo "SFRO_STATE=$dir/state"
