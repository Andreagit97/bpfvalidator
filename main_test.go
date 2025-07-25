package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func defaultConfig(versions []string, successBinary bool) *Config {
	cfg := &Config{
		KernelVersions: versions,
		OutPath:        "",
		VngPath:        "vng",
		Parallel:       1,
	}
	if successBinary {
		cfg.Cmd = "/usr/bin/true"
	} else {
		cfg.Cmd = "/usr/bin/false"
	}

	return cfg
}

// todo!: we could test different versions of the virtme-ng binary, to see if we match different error messages.
// todo!: we could test parallel execution
func TestOutput(t *testing.T) {
	const (
		wrongVersion = "v5.37.1"
		wrongName    = "wrong-name"
		validVersion = "v5.4.293"
	)

	tests := []struct {
		name            string
		cfg             *Config
		resultsExpected []result
	}{
		{
			name: "wrong-machine-name",
			cfg:  defaultConfig([]string{wrongName}, true),
			resultsExpected: []result{
				{
					version: wrongName,
					res:     missing,
				},
			},
		},
		{
			name: "wrong-machine-version",
			cfg:  defaultConfig([]string{wrongVersion}, true),
			resultsExpected: []result{
				{
					version: wrongVersion,
					res:     missing,
				},
			},
		},
		{
			name: "success",
			cfg:  defaultConfig([]string{validVersion}, true),
			resultsExpected: []result{
				{
					version: validVersion,
					res:     success,
				},
			},
		},
		{
			name: "failure",
			cfg:  defaultConfig([]string{validVersion}, false),
			resultsExpected: []result{
				{
					version: validVersion,
					res:     failure,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := run(tt.cfg)
			require.Equal(t, len(tt.resultsExpected), len(results), "results length mismatch")
			require.Equal(t, tt.resultsExpected[0].version, results[0].version, "version mismatch")
			require.Equal(t, tt.resultsExpected[0].res, results[0].res, "code mismatch.")
			t.Logf("results:\n%v", results)
		})
	}
}
