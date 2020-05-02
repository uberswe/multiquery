# MultiQuery (mq)

A simple cli tool written in Go to query multiple MySQL databases.

Sometimes I have the need to query multiple MySQL databases with the same structure. So I built this simple tool to allow me to do just that. You can connect to a mysql host directly or via an SSH tunnel.

The following query will find all the databases with the prefix `wp_` and run a select query on each database and aggregate the results.
```bash
mq --host=localhost --prefix=wp_ --query="SELECT * FROM wp_users"
```

Use the help command to read about the other parameters that are supported

```bash
mq --help
```

This tool should have no bugs but use it at your own risk. If you are concerned please review my code and feel free to fork the repository to make your own changes. Feel free to open a pull request if you would like to contribute improvements to the code.

You can also use the `--threaded` option to run concurrent queries.

SSH tunneling is supported if you specify an SSH host.

This tool tries to read ssh config files and my.cnf files when possible.