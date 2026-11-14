package skills

// environmentConfiguration teaches typed startup policy while respecting the installed env/v2 fallback semantics.
const environmentConfiguration = `
# GoForj Environment Configuration

Use github.com/goforj/env/v2 for process environment access, scoped keys, and the Project's existing environment-file lifecycle. Inspect the installed version; generated Projects currently use v2.6.1. Keep configuration in an application-owned typed provider and inject one validated snapshot into services. There is no make:env generator or extra framework manager to invent.

Load files through the established env.Load startup boundary. Load is once per process; Reload explicitly reapplies file ownership rules. Process variables override files, including explicitly empty process values. Selected files layer over .env; inspect the installed loader and App environment before changing file precedence. Do not use godotenv.Overload or os.Setenv to overwrite deployment settings from local files.

Use env.WithPrefix("DISPATCH") and its GetInt, GetDuration, GetBool, and Get methods for related keys. Fallbacks are strings, not typed Go values. GetInt/GetDuration return the fallback when parsing fails; GetBool also supports yes/no and on/off with whitespace and case normalization. Empty values use getter fallbacks. These getters do not return errors or enforce required values. For strict required integers, an invalid sentinel fallback such as "0" plus explicit range checks can reject missing and malformed values. If missing versus malformed values need different errors, read the raw value and use strconv or time parsing with deliberate diagnostics.

For example, workers := env.WithPrefix("DISPATCH").GetInt("WORKERS", "0") must be checked against the application's supported range before startup succeeds. Optional settings may deliberately use documented tolerant fallbacks instead. Do not silently apply a default where the application requires failure.

Reject missing required credentials before readiness, name the offending key without printing its value, and preserve credential bytes after checking whether they are blank. Redact credentials in String and GoString diagnostics. Ordinary env values are not a secret manager; use crypt for authenticated payload encryption and an established secret reader for managed secret lookup. Environment loading does not rotate external credentials.

Preserve adequate os.LookupEnv/strconv code for a small existing invariant or an explicitly selected configuration package. Do not migrate working configuration solely to add an import. Verify required values, numeric bounds, malformed input, intentional fallbacks, process/file precedence, explicit empty overrides, and frozen service snapshots. Use fresh processes for loader-precedence tests because the loader owns process-wide once-only state. Keep fake credentials out of diagnostic output.
`
