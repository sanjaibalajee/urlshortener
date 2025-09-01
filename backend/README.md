# url shortener backend

simple go api for shortening urls

## setup

```bash
# install deps
go mod tidy

# copy env
cp .env.example .env

# start postgres + redis (docker)
make docker-run

# run
make run
```

## makefile commands

```bash
make build        # build the app
make run          # run the app
make test         # run tests
make itest        # integration tests
make watch        # live reload with air
make docker-run   # start containers
make docker-down  # stop containers
make clean        # remove binary
```

## env

```bash
# app
PORT=8080
APP_ENV=local

# postgres (no password)
BLUEPRINT_DB_HOST=localhost
BLUEPRINT_DB_PORT=5432
BLUEPRINT_DB_DATABASE=url
BLUEPRINT_DB_USERNAME=postgres
BLUEPRINT_DB_PASSWORD=
BLUEPRINT_DB_SCHEMA=public

# redis
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_DB=0
```

## endpoints

- `POST /shorten` - create short url
- `GET /{code}` - redirect to original url

that's it 🎯
