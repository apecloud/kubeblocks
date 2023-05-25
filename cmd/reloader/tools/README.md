<h1>cue-helper</h1>

# 1. Introduction

cue-helper is a tool that automatically generating CueLang code, the code is used to verify parameters in configuration files.

# 2. Getting Started

You can get started with cue-helper, by the following methods:
* Build `cue-helper` from sources

## 2.1 Build

Compiler `Go 1.20+` (Generics Programming Support), checking the [Go Installation](https://go.dev/doc/install) to see how to install Go on your platform.

Use `make cue-helper` to build and produce the `cue-helper` binary file. The executable is produced under current directory.

```shell
$ cd kubeblocks
$ make cue-helper
```

## 2.2 Run

You can run the following command to start cue-helper once built

```shell
reloader Provides a mechanism to implement reload config files in a sidecar for kubeblocks.

Usage of ./bin/cue-helper:
  -boolean-promotion
    	enable using OFF or ON.
  -file-path string
    	The generate cue scripts from file.
  -ignore-string-default
    	ignore string default.  (default true)
  -type-name string
    	cue parameter type name.  (default "MyParameter")
  -output-prefix string
    	prefix, default: ""	

```

```shell

# apps template parameters: https://github.com/apecloud/dbaas-template-paramters

# mysql
./bin/cue-helper --file-path apps-template-paramters/wesql/mysql8.pt --type-name MysqlParameter --boolean-promotion

# pg14
./bin/cue-helper --file-path apps-template-paramters/pg14/postgresql.pt --type-name PGParameter

```


# 7. License

Reloader is under the Apache 2.0 license. See the [LICENSE](../../LICENSE) file for details.
