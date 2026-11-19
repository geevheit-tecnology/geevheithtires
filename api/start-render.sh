#!/bin/sh
set -eu

/app/migrate
exec /app/server
