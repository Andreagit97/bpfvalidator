# Relase

## Push a tag

On the main branch

```bash
git tag v0.0.1
git push origin v0.0.1
```

The CI should automatically craft a new release

## Delete a tag

```bash
git tag -d v0.0.1
git push origin --delete v0.0.1
```
