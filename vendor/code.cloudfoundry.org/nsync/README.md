nsync
=====

**Note**: This repository should be imported as `code.cloudfoundry.org/nsync`.

Keeps diego ☆NSYNC with CC

####Learn more about Diego and its components at [diego-design-notes](https://github.com/cloudfoundry-incubator/diego-design-notes)


## Development

The nsync test suite depends on ginkgo and consul. If these are not running, assuming your `$GOPATH` is
configured correctly, you may:

```
go get github.com/onsi/ginkgo/ginkgo
brew install consul
```

To run the test suite:

```
ginkgo -r .
```
