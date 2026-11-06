#!/bin/sh

set -eu

psql -v ON_ERROR_STOP=1 \
    --username "$POSTGRES_USER" \
    --dbname "$POSTGRES_DB" \
    --set=auth_user="$AUTH_USER" \
    --set=auth_password="$AUTH_PASSWORD" \
    --set=db_user="$DB_USER" \
    --set=db_password="$DB_PASSWORD" \
    --set=auth_db="$AUTH_DB" \
    --set=app_db="$APP_DB" <<'EOSQL'

CREATE USER :"auth_user" WITH PASSWORD :'auth_password';
CREATE USER :"db_user" WITH PASSWORD :'db_password';

CREATE DATABASE :"auth_db" OWNER :"auth_user";
CREATE DATABASE :"app_db" OWNER :"db_user";

EOSQL