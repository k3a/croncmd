# CronCmd

Unlike [go-crond](https://github.com/webdevops/go-crond), this
one is tiny (no web servers, prometheus, user switching, etc)
and is always going to be tiny.

Job specs are passed as command arguments.

In the future, it may be able to parse a single cronjob file
but nothing more to keep it small and simple.

## Usage

```
croncmd [flags] 'CRON SPEC' 'COMMAND' ['CRON SPEC' 'COMMAND' ...]

Flags:
  -allow-parallel-execution
        allow starting a new job even if the previous one haven't finished yet
  -shell
        force every command to run in a shell
```

Jobs are specified in pairs or positional arguments like this:

```
croncmd '@weekly' '/do/sth.sh' '0 6 * * *' '/do/sth/else.sh' 
```

For the cron spec syntax, see the [Robfig's Cron Doc](https://godoc.org/github.com/robfig/cron).

