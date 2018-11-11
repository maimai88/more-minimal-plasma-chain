# more-minimal-plasma-chain

[![GoDoc](https://godoc.org/github.com/m0t0k1ch1/more-minimal-plasma-chain?status.svg)](https://godoc.org/github.com/m0t0k1ch1/more-minimal-plasma-chain)

a Plasma chain for https://github.com/kfichter/more-minimal-plasma

## Quickstart

Please install [Docker Compose](https://docs.docker.com/compose/install) in advance.

### Build & Run

``` sh
$ cd _docker/mmpc
$ docker-compose up -d
```

### Deploy root chain contract

``` sh
$ docker-compose exec child plasma deploy --privkey 0x4f3edf983ac636a65a842ce7c78d9aa706d3b113bce9c46f30d7d21715b23b1d
```
