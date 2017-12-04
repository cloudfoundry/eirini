# Overview of Cells

A Diego Cell starts and stops applications and tasks locally,
manages containers, and reports app status and other data to the BBS and Loggregator.

# The components of a Diego Cell

## Rep

* Represents a Cell in Diego Auctions for Tasks and LRPs
* Mediates all communication between the Cell and the BBS
* Ensures synchronization between the set of Tasks and LRPs in the BBS with the containers present on the Cell
* Maintains the presence of the Cell in the BBS
* Runs Tasks and LRPs by telling the in-process Executor to create a container and RunAction recipes

## Executor

* Runs as a logical process inside the Rep
* Implements the generic Executor actions detailed in the API documentation
* Streams STDOUT and STDERR to the Metron agent running on the Cell

## Garden

* Provides a platform-independent server and clients to manage Garden containers
* Defines the garden interface for container implementation

## Metron Agent

* Forwards application logs, errors, and application and Diego metrics to the Loggregator Doppler component

[back](README.md)
