# bpfvalidator

This is a simple tool that spawns qemu machines to test a specific binary on these machines. This is particularly useful for testing eBPF programs against different kernel versions.
Under the hood it uses [virtme-ng](https://github.com/arighi/virtme-ng) tool to create qemu instancies.
Given this configuration file:

```yaml
vng_path: "vng" # absolute path or should be in PATH
bin_command: "/usr/bin/echo 'hey'" # command to run in the qemu machines
parallel: 1 # number of parallel qemu machines to spawn
out_path: "" # if provided, the report will be saved in this file
kernel_versions:
    - v5.4.293
    - v5.10.237
    - v5.15.182
```

bpfvalidator will:

- create 3 qemu machines with the kernel versions specified in the configuration file
- run the command `/usr/bin/echo 'hey'` in each of them
- wait for the command to finish
- collect the output of the command
- generate a report telling if the command passed or failed

```bash
bpfvalidator --config config.yaml
```

```txt
Report:
- v5.4.293 🟢
- v5.10.237 🟢
- v5.15.182 🟢
```

it is possible to obtain also a verbose version of the report using `--log debug` flag:

```bash
bpfvalidator --config config.yaml --log debug
```

```txt
Report:
- v5.4.293 🟢
        message: hey

- v5.10.237 🟢
        message: hey

- v5.15.182 🟢
        message: hey
```

## Report

- 🟢: the command passed
- 🔴: the command failed
- 🟡: the provided machine doesn't exist

Example

```txt
- v7.9.0 🟡 #the provided machine doesn't exists
- v5.10.237 🟢 # success
- v5.15.182 🔴 # failure
```

## Build and run locally

```bash
go build .
./bpfvalidator
```

## Github Action

This tool is also available as a GitHub Action. You can use it in your workflows to validate programs against different kernel versions.
By default the action will fail if any of the kernels fail the validation. The report with failures will be available in the action output.

```yaml
  test:
    name: test
    runs-on: ubuntu-24.04
    steps:
      - name: Checkout
        uses: actions/checkout@v4
        with:
          fetch-depth: 0
    
      - name: Run bpfvalidator
        uses: Andreagit97/bpfvalidator@main
        id: test
        with:
          cmd: "/usr/bin/echo 'OK'"
          kernels: "v5.4.293,v5.10.237,v5.15.182"
          kvm: true # supported only on amd64 github runners, not on arm64
```

if you don't want the action to fail when a kernel fails the validation, you can set the `fail_on_validation` input to `false`, and use the outputs of the action to handle the results in your workflow.
The action offers two outputs: `report` and `outcome`.

- `report`: a string containing the report of the validation.
- `outcome`: if the tool was successful or not. If the action fails, the value will be `1`, otherwise it will be `0`.

```yaml
  test:
    name: test
    runs-on: ubuntu-24.04
    steps:
      - name: Checkout
        uses: actions/checkout@v4
        with:
          fetch-depth: 0
    
      - name: Run bpfvalidator
        uses: Andreagit97/bpfvalidator@main
        id: test
        with:
          cmd: "/usr/bin/echo 'OK'"
          kernels: "v5.4.293,v5.10.237,v5.15.182"
          kvm: true
          fail_on_validation: false

      - name: Show report 
        run: cat ${{ steps.test.outputs.report }}

      - name: Show outcome
        run: echo "${{ steps.test.outputs.outcome }}"
```
