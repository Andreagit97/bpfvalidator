package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	keyConfigPath = "config"

	keyVngPath        = "vng_path"
	keyCmd            = "cmd"
	keyParallel       = "parallel"
	keyOutPath        = "out_path"
	keyReportOnly     = "report_only"
	keyKernelVersions = "kernel_versions"

	defaultConfigPath = "./config.yaml"
	defaultVngPath    = "vng" // assuming vng is in PATH
	defaultCmd        = ""    // no default command
	defaultParallel   = 1
	defaultOutPath    = ""
	defaultReportOnly = false
)

type Config struct {
	VngPath        string   `mapstructure:"vng_path"`
	Cmd            string   `mapstructure:"cmd"`
	Parallel       int      `mapstructure:"parallel"`
	OutPath        string   `mapstructure:"out_path"`
	ReportOnly     bool     `mapstructure:"report_only"`
	KernelVersions []string `mapstructure:"kernel_versions"`
}

// fileExists checks if the given path exists and is accessible
func fileExists(path string) bool {
	if !filepath.IsAbs(path) {
		var err error
		if path, err = exec.LookPath(path); err != nil {
			return false
		}
	}
	if _, err := os.Stat(path); err != nil {
		return false
	}
	return true
}

func (cfg *Config) String() string {
	return fmt.Sprintf(`Config{VngPath: "%s", Cmd: "%s", Parallel: %d, OutPath: "%s", ReportOnly: %t, KernelVersions: %v}`,
		cfg.VngPath, cfg.Cmd, cfg.Parallel, cfg.OutPath, cfg.ReportOnly, cfg.KernelVersions)
}

func (cfg *Config) validateConfig() error {
	if !fileExists(cfg.VngPath) {
		return fmt.Errorf("'vng' binary not found at '%s'", cfg.VngPath)
	}

	parts := strings.Fields(cfg.Cmd)
	if len(parts) == 0 {
		return fmt.Errorf("'cmd' is empty")
	}

	if !fileExists(parts[0]) {
		return fmt.Errorf("tested binary not found at '%s'", parts[0])
	}

	if len(cfg.KernelVersions) == 0 {
		return fmt.Errorf("'kernel_versions' cannot be empty")
	}
	return nil
}

func setupConfig() *Config {
	// The order of precedence for configuration is:
	// 1. Command line flags provided by the user
	// 2. Config file specified by the user
	// 3. Default values for the flags

	// Default values of flags are used only if the config file is not found or does not contain the key.
	pflag.StringP(keyConfigPath, "c", defaultConfigPath, "config file path")
	pflag.String(keyVngPath, defaultVngPath, "absolute path to vng binary or simple binary name if in PATH")
	pflag.String(keyCmd, defaultCmd, "path + command line of the binary to test (e.g. /usr/bin/echo 'Hello World')")
	pflag.Int(keyParallel, defaultParallel, "numer of parallel machines to run tests on")
	pflag.String(keyOutPath, defaultOutPath, "output file path for the report (if empty, prints to stdout)")
	pflag.Bool(keyReportOnly, defaultReportOnly, "if true, only prints the report without success/failure messages")
	pflag.StringSlice(keyKernelVersions, []string{}, "list of kernel versions to test (e.g. v5.4.293, v5.10)")

	// Parse and Bind flags into viper
	pflag.Parse()
	if err := viper.BindPFlags(pflag.CommandLine); err != nil {
		log.Fatalf("Error binding flags: %v", err)
	}

	// Check if config file exists
	configFile := viper.GetString(keyConfigPath)
	viper.SetConfigFile(configFile)
	if err := viper.ReadInConfig(); err != nil {
		log.Infof("Cannot find/parse config file at '%s', using defaults", configFile)
	} else {
		log.Infof("Using config file at '%s'", configFile)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		log.Fatalf("cannot unmarshal config in struct: %v", err)
	}

	log.Infof("Final configuration: %s\n", cfg.String())

	if err := cfg.validateConfig(); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}
	return &cfg
}
