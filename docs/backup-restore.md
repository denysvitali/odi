# Backup and restore

ODI stores the original page blobs separately from its OpenSearch metadata. A blob-only
backup cannot recover indexed metadata, extracted text, reminders, or share state. Browser
favorites, locally assigned tags, recent searches, and UI preferences live in localStorage
and are not included in an OpenSearch snapshot. Reindexing
is a recovery tool, not a replacement for a complete backup.

## Scope and consistency

Before operating on an archive, identify its deployment, storage root or bucket, OpenSearch
cluster/index, optional Zefix database, and backup destination. Use an encrypted backup
destination with restricted access: the search index and filesystem blobs are plaintext even
when the B2 blobs are encrypted. Keep encryption keys/passphrases and authentication secrets
in a separate protected recovery store, never in this repository or a public backup manifest.

1. Pause scanners, directory watchers, import/reindex jobs, and API writers. Wait for running
   jobs to finish, then stop the API to prevent edits, deletes, or share-view counter updates
   during the backup. Do not use `docker compose down -v`; that removes data volumes.
2. Record the application commit/image digest, OpenSearch/PostgreSQL versions, configuration
   keys (without secret values), document/page counts, and UTC backup time in the protected
   backup manifest. Record whether Zefix is used and which storage backend is configured.
3. Back up every component below while writers remain stopped. If any component fails, keep
   the previous known-good backup and mark this attempt incomplete.
4. Verify checksums and snapshot/dump completion before resuming the API and producers.

## Components

| Component | Backup method | Recovery requirement |
|---|---|---|
| Filesystem storage | Copy the entire `FS_PATH`, preserving files, directories, permissions, and ownership, while ODI writers are stopped. Include thumbnails and every page. | Restore into an empty storage root with access for the runtime user; check actual document counts and retrieval. |
| B2 or rclone storage | Use the storage provider's versioned backup/copy facility to a separate protected location. Preserve all object keys and required object versions; do not run a destructive sync. | Restore or map the copied objects to a disposable bucket/remote with the same key layout. Preserve the B2 encryption passphrase separately and verify decryption. |
| OpenSearch | Use a configured OpenSearch snapshot repository. Snapshot the configured document index (default `documents`) and `odi-shares`, plus other ODI indices actually present. Record the snapshot name and repository. | Verify the snapshot state is `SUCCESS`, with no failed shards. Restore into a compatible isolated cluster; never copy a running OpenSearch data directory. |
| PostgreSQL/Zefix (if configured) | Take a logical `pg_dump --format=custom` backup of the configured database using protected credential files. | Restore with `pg_restore` into an empty, compatible database; validate enrichment queries. The source register may be reimported if its exact version is retained. |
| Browser local state (optional) | Export favorites, local tags, recent searches, and preferences from the relevant browser profile into protected storage. Do not bundle the API token into ordinary exports. | Restore only the intended localStorage keys in the intended browser origin; server backups do not restore browser state. |
| Runtime configuration | Retain deployment configuration and CA certificates in a protected recovery store, with secrets managed separately. | Restore storage/index names, endpoint settings, TLS trust, and optional integrations before starting ODI. |

OpenSearch snapshot repositories require deployment-specific setup and access controls.
Consult the documentation for the exact deployed OpenSearch version before registering one;
a successful snapshot request alone does not prove completion. Keep backups outside the
source cluster and outside the source filesystem/bucket's failure domain. PostgreSQL dumps
and filesystem copies must also be protected because they may contain sensitive data.

## Isolated recovery drill

Use a disposable storage root/bucket, OpenSearch cluster, and database with unique endpoints
and ports. Keep production producers disabled and ensure the recovery configuration contains
no production write destinations. Start with the backed-up application and dependency
versions; upgrade only after a successful recovery. Do not publish a recovered API or enable
real scanners/watchers as part of the drill.

1. Restore blobs and metadata, then the optional Zefix dump. Use a new API token and protected
   settings; disable outbound OCR/LLM integrations unless the drill explicitly includes them.
2. Start a read-only API by leaving OCR unconfigured. Confirm `/healthz` and `/readyz` succeed
   for the configured dependencies. A liveness result alone does not validate the archive.
3. Compare restored document/page counts with the manifest. Retrieve representative documents
   and page images, search known synthetic fixtures, and confirm indexed metadata, reminders,
   and share expiry/view-limit state survived. Check favorites and locally assigned tags
   separately if browser state was backed up. Verify encrypted B2 pages can be decrypted.
4. With synthetic documents only, explicitly enable local mock OCR and test ingestion, content
   deduplication, search, and deletion. Ensure deletion targets only disposable drill data.
5. Record the backup identity, checks performed, counts, failures, and elapsed recovery time.
   Do not record document text, queries, credentials, or share tokens in ordinary logs.
6. Remove the disposable environment after retaining a protected drill report. Keep the task
   pending if counts, decryption, metadata, or document retrieval do not match expectations.

Set backup frequency and retention from the acceptable loss window and archive size. Retain
multiple independent generations and repeat this drill after storage, index-schema, or
cryptography changes. This guide describes the acceptance procedure; it does not claim that
an existing deployment has a verified backup or a successful restore.

## Interrupted ingestion or an uncertain indexing result

Keep blobs already written when OCR/indexing fails or an OpenSearch write outcome is
uncertain. An ordinary indexing failure releases its digest reservation while retaining the blob;
an interrupted process may leave a reservation behind. Do not manually delete blobs or
reservations as a cleanup step: retain the evidence needed to recover the page. Pause the affected producer, record page IDs
and error/status metadata without document content, and inspect the actual stored/indexed
state. After the dependency recovers, use `odi reindex` against the same storage backend
and index, optionally restricting it to the affected scan ID. Reindexing uses the existing
page IDs and handles either an absent reservation or a reservation owned by that same page. Verify page retrieval,
search visibility, and deduplication before resuming producers; a retry launch alone is
not successful recovery. Retain unresolved orphan blobs/reservations for investigation.
