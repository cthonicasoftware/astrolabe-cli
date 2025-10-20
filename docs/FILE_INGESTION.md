# File Ingestion Guide

The file ingestion feature allows you to import existing data files into Astrolabe for archiving, analysis, and upload to the QA backend.

## Supported Formats

### CSV (Comma-Separated Values)

Ingest CSV files with or without headers. Column data is automatically parsed into structured records.

**Features:**
- Automatic header detection
- Custom delimiters (tab, semicolon, etc.)
- Explicit column naming
- Skip header rows

**Example:**
```bash
# Basic CSV with headers
astrolabe capture file data.csv

# CSV without headers (auto-generate column names)
astrolabe capture file data.csv --no-headers

# Tab-separated values
astrolabe capture file data.tsv --delimiter '\t'

# Skip first 2 lines (e.g., comments)
astrolabe capture file data.csv --skip-lines 2

# Specify custom column names
astrolabe capture file data.csv --columns "timestamp,voltage,current"
```

### JSONL (Newline-Delimited JSON)

Ingest files containing one JSON object per line. This format is ideal for event logs and structured data exports.

**Example:**
```bash
# Auto-detected by .jsonl extension
astrolabe capture file events.jsonl

# Explicit format specification
astrolabe capture file data.ndjson --format jsonl
```

### Raw (Plain Text Logs)

Ingest unstructured text files where each line becomes a separate record.

**Features:**
- Preserves line content and metadata
- Ideal for application logs, console output, etc.

**Example:**
```bash
# Auto-detected by .log or .txt extension
astrolabe capture file app.log

# Explicit format
astrolabe capture file output.txt --format raw
```

## Command Reference

### Basic Usage

```bash
astrolabe capture file <path> [flags]
```

### Common Flags

| Flag | Description | Example |
|------|-------------|---------|
| `--format` | File format: `csv`, `jsonl`, or `raw` | `--format csv` |
| `--skip-lines` | Number of lines to skip at start | `--skip-lines 2` |
| `--test-plan` | Test plan identifier | `--test-plan "smoke-test"` |
| `--operator` | Operator name | `--operator "alice"` |
| `--location` | Test bench/location | `--location "bench-1"` |

### CSV-Specific Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--delimiter` | Field delimiter | `,` |
| `--no-headers` | CSV has no header row | `false` |
| `--columns` | Explicit column names | Auto-detect |

### Metadata Flags

Apply metadata to ingested files (same as serial capture):

```bash
astrolabe capture file data.csv \
  --operator "alice" \
  --location "bench-1" \
  --device-id "dev-001" \
  --device-firmware "v1.2.3" \
  --test-plan "regression" \
  --tag "automated" \
  --tag "nightly" \
  --attr "environment=lab"
```

## Workflow Examples

### Import Test Results from CI

```bash
# Import CSV test results
astrolabe capture file /path/to/test-results.csv \
  --format csv \
  --test-plan "ci-pipeline" \
  --operator "jenkins" \
  --tag "automated" \
  --tag "ci"

# Upload to server
astrolabe upload
```

### Migrate Historical Data

```bash
# Import old log files
for log in /archive/*.log; do
  astrolabe capture file "$log" \
    --format raw \
    --test-plan "historical-data" \
    --tag "migration"
done

# Upload all at once
astrolabe upload
```

### Import Instrument Data

```bash
# Import oscilloscope CSV export
astrolabe capture file scope-capture.csv \
  --delimiter ',' \
  --skip-lines 5 \
  --device-id "scope-001" \
  --test-plan "signal-analysis"
```

## How It Works

### Data Flow

1. **Source**: File is opened and read line-by-line (or in chunks)
2. **Normalization**: Data is parsed according to format (CSV, JSONL, raw)
3. **Storage**: Normalized records are saved to local cache
4. **Upload**: Run can be uploaded to QA backend using `astrolabe upload`

### Output Structure

Ingested files are stored in `~/.astrolabe/runs/<run-id>/`:

```
~/.astrolabe/runs/run-20241015-103045/
├── manifest.json       # Run metadata including source file info
└── data.jsonl          # Normalized records in JSONL format
```

### Record Structure

All ingested data is normalized to the standard Record format:

```json
{
  "ts": "2024-01-15T10:00:00Z",
  "seq": 1,
  "type": "csv_row",
  "payload": {
    "timestamp": "2024-01-15T10:00:00Z",
    "voltage": "3.3",
    "current": "0.5",
    "status": "OK"
  }
}
```

**Record Types:**
- `csv_row` - CSV data rows
- `sample` - JSONL objects
- `raw` - Raw log lines

## Format Auto-Detection

If `--format` is not specified, Astrolabe auto-detects based on file extension:

| Extension | Detected Format |
|-----------|----------------|
| `.csv` | csv |
| `.jsonl`, `.ndjson` | jsonl |
| `.log`, `.txt` | raw |
| Other | raw (fallback) |

## CSV Parsing Details

### With Headers (Default)

```csv
name,age,city
Alice,30,NYC
Bob,25,SF
```

**Result:**
```json
{"type": "csv_row", "payload": {"name": "Alice", "age": "30", "city": "NYC"}}
{"type": "csv_row", "payload": {"name": "Bob", "age": "25", "city": "SF"}}
```

### Without Headers

```csv
Alice,30,NYC
Bob,25,SF
```

**Command:**
```bash
astrolabe capture file data.csv --no-headers
```

**Result:**
```json
{"type": "csv_row", "payload": {"col_0": "Alice", "col_1": "30", "col_2": "NYC"}}
{"type": "csv_row", "payload": {"col_0": "Bob", "col_1": "25", "col_2": "SF"}}
```

### Custom Column Names

```bash
astrolabe capture file data.csv --columns "person,years,location"
```

**Result:**
```json
{"type": "csv_row", "payload": {"person": "Alice", "years": "30", "location": "NYC"}}
```

## Troubleshooting

### "Path is a directory, not a file"

You must specify a file path, not a directory. Use wildcards or a script to process multiple files.

### "Cannot access file: permission denied"

Ensure the file is readable:
```bash
chmod 644 /path/to/file.csv
```

### CSV parsing errors

If CSV parsing fails:
- Check delimiter with `--delimiter`
- Verify file encoding (UTF-8 expected)
- Try `--no-headers` if auto-detection fails

### Empty records

If records appear empty:
- Check file format matches `--format` flag
- Verify file isn't binary
- Use `cat` or `head` to inspect file contents

## Integration with Metadata Defaults

File ingestion respects metadata defaults from `~/.astrolabe/metadata.json`:

```json
{
  "operator": "alice",
  "location": "bench-1",
  "device": {
    "id": "dev-001"
  }
}
```

Command-line flags override these defaults.

## Performance Notes

- Large files are processed incrementally (low memory usage)
- Line-by-line processing handles files of any size
- No explicit file size limits

## Next Steps

After ingesting files:

1. **Validate**: Check the run was created correctly
   ```bash
   ls ~/.astrolabe/runs/
   cat ~/.astrolabe/runs/<run-id>/manifest.json
   ```

2. **Upload**: Send to QA backend
   ```bash
   astrolabe upload
   ```

3. **Verify**: Check upload status
   ```bash
   # Upload will show success/failure for each run
   ```

## See Also

- [Upload Guide](UPLOAD_GUIDE.md) - Uploading ingested data
- [Directory Structure](DIRECTORY_STRUCTURE.md) - Understanding the cache layout
- [Configuration](CONFIG_TUI.md) - Setting up backend connection
