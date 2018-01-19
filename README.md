# go-libp2p-stream-transport

[![](https://img.shields.io/badge/made%20by-Protocol%20Labs-blue.svg?style=flat-square)](http://ipn.io)
[![](https://img.shields.io/badge/freenode-%23ipfs-blue.svg?style=flat-square)](http://webchat.freenode.net/?channels=%23ipfs)
[![](https://img.shields.io/badge/project-IPFS-blue.svg?style=flat-square)](http://ipfs.io/)
[![standard-readme compliant](https://img.shields.io/badge/standard--readme-OK-green.svg?style=flat-square)](https://github.com/RichardLitt/standard-readme)
[![GoDoc](https://godoc.org/github.com/libp2p/go-libp2p-stream-transport?status.svg)](https://godoc.org/github.com/libp2p/go-libp2p-stream-transport)
[![Coverage Status](https://coveralls.io/repos/github/libp2p/go-libp2p-stream-transport/badge.svg?branch=master)](https://coveralls.io/github/libp2p/go-libp2p-stream-transport?branch=master)
[![Build Status](https://travis-ci.org/libp2p/go-libp2p-stream-transport.svg?branch=master)](https://travis-ci.org/libp2p/go-libp2p-stream-transport)

> Stream transport to libp2p transport upgrader

This package provides the necessary logic to upgrade [multiaddr-net][manet] connections listeners into full [libp2p-transport][tpt] connections and listeners.

To use, construct a new `Upgrader` with:

* An optional [pnet][pnet] `Protector`.
* An optional [multiaddr-net][manet] address `Filter`.
* A mandatory [stream security transport][ss].
* A mandatory [stream multiplexer transport][smux].

[tpt]: https://github.com/libp2p/go-libp2p-transport
[manet]: https://github.com/multiformats/go-multiaddr-net
[ss]: https://github.com/libp2p/go-stream-security
[smux]: https://github.com/libp2p/go-stream-muxer
[pnet]: https://github.com/libp2p/go-libp2p-interface-pnet

Note: This package largely replaces the functionality of [go-libp2p-conn](https://github.com/libp2p/go-libp2p-conn) but with half the code.

## Install

`go-libp2p-stream-transport` is a standard Go module which can be installed with:

```sh
go get github.com/libp2p/go-libp2p-stream-transport
```

Note that `go-libp2p-stream-transport` is packaged with Gx, so it is recommended to use Gx to install and use it (see the Usage section).

## Usage

This module is packaged with [Gx](https://github.com/whyrusleeping/gx). In order to use it in your own project it is recommended that you:

```sh
go get -u github.com/whyrusleeping/gx
go get -u github.com/whyrusleeping/gx-go
cd <your-project-repository>
gx init
gx import github.com/libp2p/go-libp2p-stream-transport
gx install --global
gx-go --rewrite
```

Please check [Gx](https://github.com/whyrusleeping/gx) and [Gx-go](https://github.com/whyrusleeping/gx-go) documentation for more information.

For more information about how `go-libp2p-stream-transport` is used in the libp2p context, you can see the [go-libp2p-conn](https://github.com/libp2p/go-libp2p-conn) module.

## Contribute

Feel free to join in. All welcome. Open an [issue](https://github.com/libp2p/go-libp2p-stream-transport/issues)!

This repository falls under the IPFS [Code of Conduct](https://github.com/libp2p/community/blob/master/code-of-conduct.md).

### Want to hack on IPFS?

[![](https://cdn.rawgit.com/jbenet/contribute-ipfs-gif/master/img/contribute.gif)](https://github.com/ipfs/community/blob/master/contributing.md)

## License

MIT
