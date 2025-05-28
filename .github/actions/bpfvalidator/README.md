# bpfvalidator-action

This action allows you to run the bpfvalidator tool in a GitHub Actions workflow.

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
