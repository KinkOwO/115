Read-only protocol/content fixtures, not runtime configuration or reward tables.

weekly.json, online.json, daily.json and login.json are token exports from the
installed CN client's 2026-10-06 PVF. Decrypted inner SHA256:
cb5ad788c52962838d1fb4dd2256c6f0cbc363d2837b2d2f9be85b1f38d88d38

weekly_us.json is a token export from the current local US inner PVF. SHA256:
83830f0693f1ceac104df5230782bf6527f2ca35580bfdf365eb94dcb2beb9ce

Script paths are declared by activity/catalog.go. Full tokens retain type-8
localization references omitted by the human-readable pvfinspect text export.
The runtime reads the selected PVF through gamedata.Source only.
