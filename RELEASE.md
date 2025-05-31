# Relase

## Push a tag

On the main branch

```bash
git tag v0.0.1
git push origin v0.0.1
```

The CI should automatically craft a new release

## Publish the action on the marketplace

Once the CI has created the release, you can publish the action on the GitHub Marketplace editing the release and clicking on the "Publish" button.

## Delete a tag

```bash
git tag -d v0.0.1
git push origin --delete v0.0.1
```
