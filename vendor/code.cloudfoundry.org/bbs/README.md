# BBS Server [![GoDoc](https://godoc.org/github.com/cloudfoundry/bbs?status.svg)](https://godoc.org/github.com/cloudfoundry/bbs)

**Note**: This repository should be imported as `code.cloudfoundry.org/bbs`.

API to access the database for Diego.

A general overview of the BBS is documented [here](doc).

## API

To interact with the BBS from outside of Diego, use the methods provided on the
ExternalClient interface, documented [here](https://godoc.org/github.com/cloudfoundry/bbs#ExternalClient).

Components within Diego may use the full [Client interface](https://godoc.org/github.com/cloudfoundry/bbs#Client) to modify internal state.

## Code Generation

The protobuf models in this repository require version 3 of the `protoc` compiler.


### OSX

On Mac OS X with [Homebrew](http://brew.sh/), run the following to install it:

```
brew install protobuf
```

### Linux

1. Download a zip archive of the latest protobuf release from [here](https://github.com/google/protobuf/releases).
1. Unzip the archive in `/usr/local`.
1. `chmod a+x /usr/local/bin/protoc` to make sure you can use the binary.

> If you already have an older version of protobuf installed, you must
> uninstall it first by running `brew uninstall protobuf`

Install the `gogoproto` compiler by running:

```
go install github.com/gogo/protobuf/protoc-gen-gogoslick
```

Run `go generate ./...` from the root directory of this repository to generate code from the `.proto` files as well as to generate fake implementations of certain interfaces for use in test code.


### Generating ruby models for BBS models

The following documentation assume the following versions:

1. [protoc](https://developers.google.com/protocol-buffers/docs/downloads) `> v3.0.0`
2. [ruby protobuf gem](https://github.com/ruby-protobuf/protobuf) `> 3.6.12`

Run the following commands from the `models` directory to generate `.pb.rb`
files for the BBS models:

1. `gem install protobuf`
2. `cp $(which protoc-gen-ruby){,2}`
3. `protoc -I$GOPATH/src --proto_path=. --ruby2_out=/path/to/ruby/files *.proto`

**Note** Replace `/path/to/ruby/files` with the desired destination of the
`.pb.rb` files. That directory must exist before running this command.

**Note** The above steps assume that
`github.com/gogo/protobuf/gogoproto/gogo.proto` is on the `GOPATH`.

**Note** Since protoc v3 now ships with a ruby generator, the built-in
generator will mask the gem's binary. This requires a small hack in order to be
able to use the protobuf gem, the hack is simply to rename the protobuf gem's
binary to be `ruby2` and ask protoc to generate `ruby2` code which will force
it to use the binary. For more information please
[read this github issue](https://github.com/ruby-protobuf/protobuf/issues/341)

## SQL

See the instructions in [Running the Experimental SQL Unit Tests](https://github.com/cloudfoundry/diego-release/blob/develop/CONTRIBUTING.md#running-the-experimental-sql-unit-tests)
for testing against a SQL backend
