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