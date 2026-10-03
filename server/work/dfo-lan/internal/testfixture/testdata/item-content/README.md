# Historical item-content audit snapshots

These gzip files preserve the complete historical `item-materials`, `item-period-tags`,
and `skin-storage-items` JSON catalogs for migration tests. `testfixture.ItemContentPath`
checks each decompressed SHA256, changes only the temporary copy's source checksum, and
returns that copy for typed-field comparison with a current native PVF projection.
The snapshots are test inputs only; runtime code must load these domains from PVF.
