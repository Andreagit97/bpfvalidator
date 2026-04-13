// Copyright 2026 Andrea Terzolo
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

// buildVngCmdline builds vng command line.
func buildVngCmdline(vngPath, BinCommand string, vmCfg *VMConfig) []string {
	kernelVersion := vmCfg.KernelVersion
	var vngArgs []string
	if vmCfg.VngArgs != "" {
		vngArgs = strings.Split(vmCfg.VngArgs, " ")
	}
	cmdline := []string{vngPath, "-r", kernelVersion}
	cmdline = append(cmdline, vngArgs...)
	cmdline = append(cmdline, "--", BinCommand)
	return cmdline
}

// runVng executes the vng command for a specific kernel version and returns true if exit code is 0
func runVng(ctx context.Context, vngPath, BinCommand string, vmCfg *VMConfig) result {
	kernelVersion := vmCfg.KernelVersion
	cmdline := buildVngCmdline(vngPath, BinCommand, vmCfg)
	log.Infof("Running command `%v`\n", cmdline)
	defer log.Infof("Command complete `%v`\n", cmdline)

	cmd := exec.CommandContext(ctx, cmdline[0], cmdline[1:]...)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	err := cmd.Run()
	if err == nil {
		return result{version: kernelVersion, res: success, message: "Stdout:\n" + stdoutBuf.String()}
	}

	stderrStr := stderrBuf.String()
	if strings.Contains(stderrStr, defaultMissingKernelVersion) || strings.Contains(stderrStr, defaultWrongFormat) {
		return result{version: kernelVersion, res: missing, message: "Stdout:\n" + stdoutBuf.String() + "\nStderr:\n" + stderrBuf.String()}
	}

	return result{version: kernelVersion, res: failure, message: "Stdout:\n" + stdoutBuf.String() + "\nStderr:\n" + stderrBuf.String()}
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
			messages += fmt.Sprintf("- %s %s:\n%s\n", icon, r.version, r.message)
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

	results := make([]result, len(cfg.VMConfigs))

	// Launch vng commands concurrently for each VM configuration.
	for i, vmCfg := range cfg.VMConfigs {
		select {
		case <-ctx.Done():
			results[i] = result{version: vmCfg.KernelVersion, res: cancelled, message: "Cancelled"}
			continue
		default:
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, vmCfg *VMConfig) {
			defer wg.Done()
			defer func() { <-sem }()
			results[idx] = runVng(ctx, cfg.VngPath, cfg.Cmd, vmCfg)
		}(i, &vmCfg)
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
