# Examples

This directory contains small examples for the supported CLI workflow.

## Files

- `sample-note.html`: printable HTML for `-print-html` or `-print-html-img`
- `quickstart.sh`: example commands using either `config.yaml` or environment variables

## Example usage

```bash
./memobird -config config.yaml -print-html-img "$(cat examples/sample-note.html)"
make print-html-img HTML="$(cat examples/sample-note.html)"
```
