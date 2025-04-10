package main

import (
	"bufio"
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
	fmt.Fprintf(os.Stderr, "Usage: %s [flags] [ /path/to/cronfile | ('CRON SPEC' 'COMMAND' ['CRON SPEC' 'COMMAND' ...]) ]\n\nFlags:\n", os.Args[0])
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

// Instruction represents a pair of cron specification and command.
type Instruction struct {
	CronSpec string
	Command  string
}

func parseCrontab(path string, withUsers bool) ([]Instruction, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Initialize a scanner to read the file line by line
	scanner := bufio.NewScanner(file)

	var instructions []Instruction

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Ignore empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Split the line into fields
		fields := strings.Fields(line)

		// Check for special cron specifications
		if len(fields) >= 2 && strings.HasPrefix(fields[0], "@") {
			// Handle special formats like @daily, @hourly, etc.
			cronSpec := fields[0]
			command := strings.Join(fields[1:], " ")
			if withUsers {
				// The 2nd field is the user, which we ignore here
				command = strings.Join(fields[2:], " ")
			}

			instructions = append(instructions, Instruction{
				CronSpec: cronSpec,
				Command:  command,
			})
		} else if len(fields) >= 6 {
			// Handle standard 5-field cron specifications
			cronSpec := strings.Join(fields[:5], " ")
			command := strings.Join(fields[5:], " ")
			if withUsers {
				// The 6th field is the user, which we ignore here
				command = strings.Join(fields[6:], " ")
			}

			instructions = append(instructions, Instruction{
				CronSpec: cronSpec,
				Command:  command,
			})
		}
	}

	// Check for errors during scanning
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return instructions, nil
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

	var instructions []Instruction

	// parse jobs from args
	args := flag.Args()
	if len(args) == 0 {
		// try to parse system cronfile
		if _, err := os.Stat("/etc/crontab"); err != nil {
			l.Printf("WARNING: No jobs specified and /etc/crontab is not available.")
		} else {
			instructions, err = parseCrontab("/etc/crontab", true)
			if err != nil {
				l.Fatalf("Cannot parse /etc/contab: %v", err)
			}
		}
	} else if len(args) == 1 {
		var err error
		instructions, err = parseCrontab(args[0], args[0] == "/etc/crontab")
		if err != nil {
			l.Fatalf("Cannot parse %s: %v", args[0], err)
		}
	} else {
		if len(args)%2 != 0 {
			l.Printf("ERROR: Odd numbers of positional arguments.")
			printUsage()
			os.Exit(1)
		}

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

			instructions = append(instructions, Instruction{
				CronSpec: cronspec,
				Command:  cmdspec,
			})
		}
	}

	reShellDetect := regexp.MustCompile("[$<>&|;#`]")

	for _, inst := range instructions {
		useShell := *forceShell
		if !useShell && reShellDetect.MatchString(inst.Command) {
			useShell = true
		}

		_, err := cr.AddJob(inst.CronSpec, &job{
			logger:      l,
			preserveEnv: true,
			command:     inst.Command,
			useShell:    useShell,
		})
		if err != nil {
			l.Fatalf("Bad cron spec: %v", err)
		}
	}

	l.Printf("Running with %d jobs defined", len(cr.Entries()))

	cr.Run()
}
