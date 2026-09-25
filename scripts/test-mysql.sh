#!/bin/sh
# Uses a disposable instance, never the MySQL service already running locally.
set -eu
command -v mysqld >/dev/null
command -v mysqladmin >/dev/null
work=$(mktemp -d /tmp/omr-integration-XXXXXX)
server_pid=''
cleanup() {
  if [ -n "$server_pid" ]; then
    mysqladmin --no-defaults --socket="$work/mysql.sock" -uroot shutdown >/dev/null 2>&1 || kill "$server_pid" 2>/dev/null || true
    wait "$server_pid" 2>/dev/null || true
  fi
  rm -rf "$work"
}
trap cleanup EXIT HUP INT TERM
port=$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')
mysqld --no-defaults --initialize-insecure --datadir="$work/data" --log-error="$work/init.log"
mysqld --no-defaults --datadir="$work/data" --socket="$work/mysql.sock" --port="$port" --bind-address=127.0.0.1 --mysqlx=0 --pid-file="$work/mysql.pid" --log-error="$work/mysql.log" &
server_pid=$!
i=0
until mysqladmin --no-defaults --socket="$work/mysql.sock" -uroot ping >/dev/null 2>&1; do
  i=$((i + 1))
  if [ "$i" -gt 100 ] || ! kill -0 "$server_pid" 2>/dev/null; then
    cat "$work/mysql.log"
    exit 1
  fi
  sleep 0.2
done
OMR_TEST_MYSQL="127.0.0.1:$port" go test -race -count=1 "$@" ./...
