tps
===

**Note**: This repository should be imported as `code.cloudfoundry.org/tps`.

**Note**: The tps listener has been removed, its functionality is in cloud_controller_ng


the process status reporter

![](http://i.imgur.com/G0MB1s4.png)

### Running Unit Tests

Running TPS specs locally requires postgres running with the correct configuration.

1. Install Postgres (version 9.4 or higher is required):

```
$ brew install postgresql
```

By default, brew installs Postgres to use /usr/local/var/postgres as its data directory, and the instructions below assume that.

2. Run postgres in daemon mode:

```
$ pg_ctl -D /usr/local/var/postgres -l /usr/local/var/postgres/server.log start
```

3. Create the `locket` database with a specific user

Enter `locket_pw` when prompted for the user's password.
```
$ createdb locket
$ createuser -d -P -r -s locket
```

### Resources

Learn more about Diego and its components at [diego-design-notes](https://github.com/cloudfoundry-incubator/diego-design-notes)
