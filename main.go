package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

const (
	defaultConfigPath  = "config.yaml"
	successIcon        = "🟢"
	missingMachineIcon = "🟡"
	failureIcon        = "🔴"
	cancelledIcon      = "⚠️"
	// this could change between virtme-ng versions, we need to update it or create a match version<->error.
	// Tested against virtme-ng 1.33+93.g62b9b2f
	defaultMissingKernelVersion = "failed to retrieve content"
	defaultWrongFormat          = "does not exist"
)

type code uint8

const (
	success code = iota
	failure
	missing
	cancelled
)

type result struct {
	version string
	res     code
	message string
}

func (r result) String() string {
	return fmt.Sprintf("%s -> version: %s, message: '%s'", convertResToIcon(r.res), r.version, r.message)
}

// Config represents the structure of the YAML configuration file
type Config struct {
	VngPath        string   `yaml:"vng_path"`
	BinCommand     string   `yaml:"bin_command"`
	Parallel       int      `yaml:"parallel"`
	OutPath        string   `yaml:"out_path"`
	KernelVersions []string `yaml:"kernel_versions"`
}

// fileExists checks if the given path exists and is accessible
func fileExists(path string) bool {
	if !filepath.IsAbs(path) {
		var err error
		if path, err = exec.LookPath(path); err != nil {
			log.Errorf("'%s' is not absolute. error looking for path: %v", path, err)
			return false
		}
	}
	if _, err := os.Stat(path); err != nil {
		return false
	}
	return true
}

// runVng executes the vng command for a specific kernel version and returns true if exit code is 0
func runVng(ctx context.Context, vngPath, BinCommand, version string) result {
	cmdline := []string{vngPath, "-r", version, "--", BinCommand}
	log.Debugf("Running command `%v`\n", cmdline)
	defer log.Debugf("Command complete `%v`\n", cmdline)

	cmd := exec.CommandContext(ctx, cmdline[0], cmdline[1:]...)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	err := cmd.Run()
	if err == nil {
		return result{version: version, res: success, message: stdoutBuf.String()}
	}

	stderrStr := stderrBuf.String()
	if strings.Contains(stderrStr, defaultMissingKernelVersion) || strings.Contains(stderrStr, defaultWrongFormat) {
		return result{version: version, res: missing, message: stderrStr}
	}

	return result{version: version, res: failure, message: stderrStr}
}

func convertResToIcon(res code) string {
	switch res {
	case success:
		return successIcon
	case failure:
		return failureIcon
	case missing:
		return missingMachineIcon
	case cancelled:
		return cancelledIcon
	default:
		panic(fmt.Sprintf("Unknown result code: %d", res))
	}
}

func printReport(results []result, outfile string) {
	report := "\nReport:\n"
	for _, r := range results {
		icon := convertResToIcon(r.res)
		if log.GetLevel() < log.DebugLevel {
			report += fmt.Sprintf("- %s %s\n", r.version, icon)
		} else {
			report += fmt.Sprintf("- %s %s\n\tmessage: %s\n", r.version, icon, r.message)
		}
	}

	if outfile != "" {
		if err := os.WriteFile(outfile, []byte(report), 0644); err != nil {
			log.Fatalf("Error writing report to file: %v\n", err)
		}
	} else {
		fmt.Print(report)
	}

}

func run(cfg *Config) []result {
	// Verify that the binaries exist
	if !fileExists(cfg.VngPath) {
		log.Fatalf("'vng' binary not found at %s\n", cfg.VngPath)
	}

	parts := strings.Fields(cfg.BinCommand)
	if len(parts) == 0 {
		log.Fatalf("`bin_command` is empty")
	}

	if !fileExists(parts[0]) {
		log.Fatalf("tested binary not found at %s\n", cfg.VngPath)
	}

	if len(cfg.KernelVersions) == 0 {
		log.Fatalf("No kernel versions specified in the configuration file\n")
	}

	// Handle graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Fprintln(os.Stderr, "\nInterrupt received, shutting down...")
		cancel()
	}()

	sem := make(chan struct{}, cfg.Parallel)
	var wg sync.WaitGroup

	results := make([]result, len(cfg.KernelVersions))

	// Launch vng commands concurrently for each kernel version
	for i, ver := range cfg.KernelVersions {
		select {
		case <-ctx.Done():
			results[i] = result{version: ver, res: cancelled, message: "Cancelled"}
			continue
		default:
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, version string) {
			defer wg.Done()
			defer func() { <-sem }()
			results[idx] = runVng(ctx, cfg.VngPath, cfg.BinCommand, version)
		}(i, ver)
	}

	wg.Wait()
	return results
}

func main() {
	cfgPath := flag.String("config", defaultConfigPath, "Path to the YAML configuration file")
	logLevel := flag.String("log", "info", "Log level (debug, info)")
	flag.Parse()
	// initially set the log level to info so we are sure that we can see errors
	log.SetLevel(log.InfoLevel)

	switch *logLevel {
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	default:
		log.Fatalf("Invalid log level: %s\n", *logLevel)
	}

	// Read the configuration file
	data, err := os.ReadFile(*cfgPath)
	if err != nil {
		log.Fatalf("error reading config file: %v", err)
	}

	// Unmarshal YAML into Config struct
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("error parsing YAML: %v\n", err)
	}
	reports := run(&cfg)
	printReport(reports, cfg.OutPath)
}
