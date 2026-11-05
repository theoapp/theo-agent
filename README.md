## Theo agent

theo-agent is the command line program used to fetch `authorized_keys` from [theo](https://github.com/theoapp/theo-node) server

### Documentation

Available at [theoapp.readthedocs.io](https://theoapp.readthedocs.io/en/latest/)


### Build
Building from sources requires Go 1.26 or later and a full git clone (tags are used to compute the version).

To run the tests and build theo-agent for the platform you are on:

```
make build
```

To cross-build for another platform, set `GOOS` and `GOARCH` (and `GOARM` for 32-bit ARM):

```
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 make -e buildx
```

The binaries are written to the `build` directory.
