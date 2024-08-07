# GO Migrations

Db migrations library

# TODO

- implement locking to not allow concurrency when running migrations. Only one migration run at 
  a time
- validation mechanism for checking if all files from migrations folder have been loaded & alert
user if not all have been loaded (hard stop/disable any migrations run)
- validation for executions inconsistencies: like a file has been generated with an old
version number compared to latest executed  number
- add to docs that users should not change the name of migration files (at least not the version number)
- think about context and timeouts where used


No locking is done while persisting migration execution changes in the repository.
This is due to the fact that, in distributed systems, it's hard to manage cluster level
locking (for example, at the time of writing, year 2024, MariaDB does not support advisory locking or table locks with Galera Cluster).
It is preferred to give locking control to the caller, for example, if automatic migrations
are run via a process manager or scheduler, make sure they do not allow concurrent or parallel
runs.
Also, it is best to write your migrations to be idempotent.
The library was built with flexibility in mind, so you are free to add anything in the
Up() or Down() migration functions. For example, use sql "... if not exists ..." clause to make
a table creation idempotent. If big tables need to be populated, use transactions or custom
checkpoints for data changes to allow retries from a checkpoint if part of the batched queries
failed.  
  

When bootstrapping the cli, it is advised to use different db handles, one for the migrations
repository and another for your migration files (if you need any). This is due to the fact that,
in the SQL database scenarios, some features like `LOCK TABLES`, if used in the migration files, may conflict with the migrations repository queries. So either you make sure you are doing some cleanup in Up(), Down() migration functions,if needed, or, use different db handles, connections. 