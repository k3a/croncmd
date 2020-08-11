package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/mattn/go-shellwords"

	"github.com/robfig/cron/v3"
)

var (
	buildstamp             = "unknown"
	allowParallelExecution = flag.Bool("allow-parallel-execution", false, "allow starting a new job even if the previous one haven't finished yet")
	forceShell             = flag.Bool("shell", false, "force every command to run in a shell")
)

func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [flags] 'CRON SPEC' 'COMMAND' ['CRON SPEC' 'COMMAND' ...]\n\nFlags:\n", os.Args[0])
	flag.PrintDefaults()
}

type job struct {
	logger      *log.Logger
	preserveEnv bool
	useShell    bool
	command     string
}

func (j *job) Run() {
	var cmd *exec.Cmd

	if j.useShell {
		j.logger.Printf("\"%s\" in shell", j.command)
		cmd = exec.Command("/bin/sh", "-c", j.command)
	} else {
		j.logger.Printf("\"%s\"", j.command)
		args, err := shellwords.Parse(j.command)
		if err != nil {
			j.logger.Printf("ERROR: Bad command spec: %v", err)
			return
		}
		if len(args) == 0 {
			j.logger.Printf("ERROR: Bad or empty command spec '%s'", j.command)
			return
		}
		cmd = exec.Command(args[0], args[1:]...)
	}

	if j.preserveEnv {
		cmd.Env = os.Environ()
	}

	// exec job
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	start := time.Now()
	err := cmd.Run()
	if err != nil {
		j.logger.Printf("ERROR: \"%s\" did not run successfuly: %v", j.command, err)
	} else {
		j.logger.Printf("\"%s\" run for %s", j.command, time.Since(start))
	}
}

func main() {
	flag.Usage = printUsage
	flag.Parse()

	// setup logger
	l := log.New(os.Stderr, "croncmd: ", log.LstdFlags|log.Lmsgprefix)
	logger := cron.PrintfLogger(l)

	// print ver
	l.Printf("croncmd build %s", buildstamp)

	// prepare cron opts
	chain := []cron.JobWrapper{
		cron.Recover(logger),
	}

	if !*allowParallelExecution {
		chain = append(chain, cron.SkipIfStillRunning(logger))
	}

	// new cron
	cr := cron.New(cron.WithChain(chain...))

	// parse jobs from args
	args := flag.Args()
	if len(args) == 0 {
		l.Printf("WARNING: No jobs specified.")
	}

	if len(args)%2 != 0 {
		l.Printf("ERROR: Odd numbers of positional arguments.")
		printUsage()
		os.Exit(1)
	}

	reShellDetect := regexp.MustCompile(`$|<|>|&&|\|\|`)

	for n := 0; n < len(args); n += 2 {
		cronspec := strings.TrimSpace(args[n])
		cmdspec := strings.TrimSpace(args[n+1])

		if cronspec == "" {
			l.Printf("WARNING: Empty cronspec for command %s, skipping", cmdspec)
			continue
		}

		if cmdspec == "" {
			l.Printf("WARNING: Empty command for cronspec %s, skipping", cronspec)
			continue
		}

		useShell := *forceShell
		if !useShell && reShellDetect.MatchString(cmdspec) {
			useShell = true
		}

		_, err := cr.AddJob(cronspec, &job{
			logger:      l,
			preserveEnv: true,
			command:     cmdspec,
			useShell:    useShell,
		})
		if err != nil {
			l.Fatalf("Bad cron spec: %v", err)
		}
	}

	l.Printf("Running with %d jobs defined", len(cr.Entries()))

	// _, err := cr.AddFunc("* * * * *", func() { //@every 1m
	// 	t := time.Now()
	// 	tstr := t.Format(time.RFC3339)

	// 	fmt.Printf("start %v\n", tstr)
	// 	if t.Minute()%5 == 0 {
	// 		panic("test panic every 5m")
	// 	}
	// 	time.Sleep(time.Minute + 30*time.Second)
	// 	fmt.Printf("finish of %v\n", tstr)
	// })

	// if err != nil {
	// 	panic(err)
	// }

	cr.Run()
}
