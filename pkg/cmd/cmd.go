package cmd

/*
  A simple package for adding subcommands in a single binary

  Commands are reistered in "Init()" functions or in main().
*/
import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Nordix/xcluster-cni/pkg/log"
	"github.com/go-logr/logr"
)

var cmds = make(map[string]func(ctx context.Context, args []string) int)

// Register Registers a command as a string associated with a "main alike"
// function.
func Register(cmd string, fn func(ctx context.Context, args []string) int) {
	// Since this is supposed to be called from Init() or main(), so
	// syncronization is NOT necessary
	cmds[cmd] = fn
}

// Run Parses the command line arguments and calls the selected subcommand.
// "-log*" flags may be passed *before* the subcommand. A logr
// logger is added to the context passed to the supcommands.
func Run(version string) int {
	// Parse the -loglevel flag and get the arguments
	logLevel := flag.String("loglevel", "", "debug|trace or an int")
	logFile := flag.String("logfile", "", "Logs are printed to this file")
	flag.Parse()
	args := flag.Args()
	if len(args) < 1 {
		// No subcommand is specified. Print a help text and quit
		fmt.Println("xcluster-cni [options] subcommand [options]")
		flag.CommandLine.SetOutput(os.Stdout)
		flag.PrintDefaults()
		fmt.Println("Subcommands:")
		for k := range cmds {
			fmt.Println("  ", k)
		}
		return 0
	}

	// Get log-file and level. Command line options has precedence
	file := os.Getenv("LOG_FILE")
	if *logFile != "" {
		file = *logFile
	}
	if file == "" {
		file = "stderr"
	}
	lvl := os.Getenv("LOG_LEVEL")
	if *logLevel != "" {
		lvl = *logLevel
	}

	// Create a context with logger
	zaplogger, err := log.ZapLogger(file, lvl)
	if err != nil {
		panic(err)
	}
	ctx, cancel := signal.NotifyContext(
		context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	ctx = log.NewContext(ctx, zaplogger)

	logger := logr.FromContextOrDiscard(ctx)
	if cmd, ok := cmds[args[0]]; ok {
		logger.V(1).Info("command", "args", args)
		return cmd(ctx, args)
	}
	logger.Info("Not found", "command", args[0])
	return 1
}
