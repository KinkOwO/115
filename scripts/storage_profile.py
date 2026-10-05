"""Which storage engine a runtime/storage/local.json selects.

This is the Python half of one rule; the Go authority is
`internal/database.engineForConfig` and the server-side launcher copy lives in
`server/work/dfo-lan/scripts/launch_local.py`. All of them read the same file and must
answer the same way: when a launcher starts one engine while the server opens the other,
the player sees a running PostgreSQL and an empty SQLite file, which reaches them as
"my account is gone" (2026-10-05, pgsql 端无法登录).

The rule, in order:
  1. an explicit `driver` always wins;
  2. otherwise a named `postgres_dsn` means PostgreSQL - the documented default engine, and
     the only safe reading of a configuration that names a DSN;
  3. otherwise `sqlite_path` means SQLite (the shape the 20261004 upgrade package's
     migration tool writes, with no driver field at all);
  4. a configuration that names neither stays on PostgreSQL, whose own open path reports
     the incomplete configuration instead of inventing one.

The table in scripts/test_environment_storage.py and the one in
server/work/dfo-lan/internal/database/engine_selection_test.go are the same table on
purpose: they are the mechanical check that the two languages have not drifted apart.
"""


def storage_driver(cfg):
    """Return the effective driver name ("sqlite" or "postgres") for a storage config."""
    driver = str(cfg.get("driver", "") or "").strip().lower()
    if driver:
        return driver
    if str(cfg.get("postgres_dsn", "") or "").strip():
        return "postgres"
    if str(cfg.get("sqlite_path", "") or "").strip():
        return "sqlite"
    return "postgres"
