# goimport-rewrite
Tool to rewrite some go import paths. The majority of this code was ripped from the `go fix` tool with some modifications then to do arbitrary import rewriting and not any of the other fix code.

## Usage

```text
usage: goimport-rewrite -r <import path>:<new import path> [-r <import path>:<new import path>] [path ...]

The list of paths may be single files or directories. If directories all .go files within that directory will be processed

  -r value
        Import to rewrite. Expected format is '<old path>:<new path>'.

```

Typical CLI Invocation:

```sh
goimport-rewrite -r github.com/foo/bar:github.com/bar/foo -r code.google.com/this/is/gone:github.com/new/location .
```
