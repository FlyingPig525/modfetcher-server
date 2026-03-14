# modfetcher-server

This is the server for the [Mod Fetcher](https://github.com/FlyingPig525/modfetcher) geode mod.
It is a simple go server meant only to store and send data about users' mods.

### Building & Running

To build and run this repository you need:
- golang 1.26

Clone the repository:

```shell
git clone https://github.com/FlyingPig525/modfetcher-server.git
```

Build:

```shell
go build github.com/FlyingPig525/modfetcher-server
```

Run:

```shell
sudo ./modfetcher-server # sudo is optional, but may be required to bind a port
```

To bind a different port, open `main.go` and modify the 2nd to last line:
- Bind port 8080: `log.Fatal(http.ListenAndServe(":80", nil))` -> `log.Fatal(http.ListenAndServe(":8080", nil))`

#### Or use Docker

```shell
docker compose up --build
```

#### Logs

Upon execution, the software logs output to stdout and 2 files. Log files can be found in the
`logs` directory. The most recent log can always be found in `recent.log`.