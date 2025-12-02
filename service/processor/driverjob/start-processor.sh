#!/bin/sh

cd /app/driver/cmd/cmd_/cmd.runfiles/_main
# prepare
/app/driver/cmd/cmd_/cmd -processor-service=test-processor-server.test:10020 \
  -rpcnode-service=test-rpcnode-server.test:18010 \
  -cache-dir=/tmp/sentio/cache \
  -prepare-processor-env-only=true \
  -chains-config=/etc/sentio/chains-config.json \
  -timescale-db-config=/etc/sentio/timescale_db_config.yaml \
  -processor-id=0XhWA854 \
  -use-pnpm=true \
  -clickhouse-config-path=/etc/sentio/clickhouse_config.yaml

# prepare2: from /etc/sentio/processor-prepare.sh

chmod 777 /tmp/sentio/cache /data && rm -rf /data/.test-write && echo
"$(date) test write /data/.test-write" > /data/.test-write && cat
/data/.test-write

mkdir -p /tmp/sentio/cache/.pnpm-store/dumps && chmod 777
/tmp/sentio/cache/.pnpm-store/dumps

TARGET_PATH=$(tail -n 1 /tmp/sentio/cache/.processor-path)

if [[ "$TARGET_PATH" == */main ]]; then
  chmod +x "$TARGET_PATH"
fi

# start processor: from /etc/sentio/processor-launcher.sh

TARGET_DIR=$(head -n 1 /tmp/sentio/cache/.processor-path)
TARGET_PATH=$(tail -n 1 /tmp/sentio/cache/.processor-path)
CHAINS_CONFIG_FILE=${TARGET_DIR}/chains-config.json
if [[ "$TARGET_PATH" == */main ]]; then
  exec "$TARGET_PATH" "$@" --chains-config=$CHAINS_CONFIG_FILE
else
  exec /usr/bin/env node --inspect=0.0.0.0:9229 $TARGET_DIR/node_modules/.bin/processor-runner "$@" --chains-config=$CHAINS_CONFIG_FILE $TARGET_PATH
fi
