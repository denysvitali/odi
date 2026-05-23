# ODI Documentation

## API Specification

- [`openapi.yaml`](openapi.yaml) — OpenAPI 3.1 spec for the REST API.
  View interactively with:

  ```bash
  npx @redocly/cli preview-docs docs/openapi.yaml
  ```

## Project Guides

- [`../README.md`](../README.md) — Quickstart, environment variables, and architecture overview.
- [`../CLAUDE.md`](../CLAUDE.md) — Codebase layout and common commands for contributors.

## Dependency Notes

- `gopkg.in/yaml.v2` is pulled in transitively by `pdfcpu`. We cannot remove it
  until `pdfcpu` upgrades to `yaml.v3`. `yaml.v3` is already present for direct
  use.
