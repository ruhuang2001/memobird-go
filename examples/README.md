# Examples

This directory contains small, copy-paste-friendly examples for the current CLI workflow.

## Files

- `sample-note.html` — simple printable HTML content for `-print-html` or `-print-html-img`
- `quickstart.sh` — example shell commands using either `config.yaml` or environment variables

## Example usage

```bash
# Print bundled sample HTML in image mode
./memobird -config config.yaml -print-html-img "$(cat examples/sample-note.html)"

# Or with make
make print-html-img HTML="$(cat examples/sample-note.html)"
```
