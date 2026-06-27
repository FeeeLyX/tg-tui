# Release Guide

## v0.1.0 checklist

1. Verify clean workspace:

```bash
git status
```

2. Run release checks:

```bash
go test ./...
VERSION=v0.1.0
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo dev)
DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)
mkdir -p bin
go build -ldflags "-X 'main.version=$VERSION' -X 'main.commit=$COMMIT' -X 'main.date=$DATE'" -o bin/tg-tui .
./bin/tg-tui --version
```

3. Review changelog and README.

4. Commit release artifacts:

```bash
git add README.md CHANGELOG.md LICENSE RELEASE.md main.go go.mod
git commit -m "release: prepare v0.1.0"
```

5. Tag release:

```bash
git tag -a v0.1.0 -m "v0.1.0"
git push origin dev --tags
```

6. Create GitHub release from tag `v0.1.0` and include highlights from `CHANGELOG.md`.
