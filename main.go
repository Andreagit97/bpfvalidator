package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	log "github.com/sirupsen/logrus"
)

const (
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

// runVng executes the vng command for a specific kernel version and returns true if exit code is 0
func runVng(ctx context.Context, vngPath, BinCommand, version string) result {
	cmdline := []string{vngPath, "-r", version, "--", BinCommand}
	log.Infof("Running command `%v`\n", cmdline)
	defer log.Infof("Command complete `%v`\n", cmdline)

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

func printReport(results []result, outfile string, reportOnly bool) bool {
	res := true
	messages := "\n# Messages\n"
	report := "\n# Report\n|version|outcome|\n|-|-|\n"
	for _, r := range results {
		if r.res != success {
			res = false
		}
		icon := convertResToIcon(r.res)

		if !reportOnly {
			messages += fmt.Sprintf("- %s %s -> %s\n", icon, r.version, r.message)
		}
		report += fmt.Sprintf("|%s|%s|\n", r.version, icon)
	}

	if !reportOnly {
		report = fmt.Sprintf("%s\n%s", messages, report)
	}

	if outfile != "" {
		if err := os.WriteFile(outfile, []byte(report), 0644); err != nil {
			log.Fatalf("Error writing report to file: %v\n", err)
		}
	} else {
		fmt.Print(report)
	}
	return res
}

func run(cfg *Config) []result {
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
			results[idx] = runVng(ctx, cfg.VngPath, cfg.Cmd, version)
		}(i, ver)
	}

	wg.Wait()
	return results
}

func main() {
	log.SetLevel(log.InfoLevel)
	cfg := setupConfig()
	reports := run(cfg)
	if ok := printReport(reports, cfg.OutPath, cfg.ReportOnly); !ok {
		os.Exit(1)
	}
}
