# GO Migrations

Db migrations library

# TODO

- implement locking to not allow concurrency whenr unning migrations. Only one migration run at a time
- validation mechanism for checking if all files from migrations folder have been loaded & alert
user if not all have been loaded (hard stop/disable any migrations run)
- validation for executions inconsistencies: like a file has been generated with an old
version number compared to latest executed  number
- add to docs that users should not change the name of migration files (at least not the version number)
- think about context and timesouts where used


No locking is done while persisting migration execution changes in the repository.
This is due to the fact that, in distributed systems, it's hard to manage cluster level
locking (for example, at the time of writing, year 2024, MariaDB does not support advisory locking or table locks with Galera Cluster).
It is prefered to give locking control to the caller, for example, if automatic migrations
are run via a process manager or scheduler, make sure they do not allow concurrent or paralel
runs.
Also, it is best to write your migrations to be idempotent.
The library was built with flexilibility in mind, so you are free to add anything in the
Up() or Down() migration functions. For example, use sql "... if not exists ..." clause to make
a table creation idempotent. If big tables need to be populated, use transactions or custom
checkpoints for data changes to allow retries from a checkpoint if part of the batched queries
failed.