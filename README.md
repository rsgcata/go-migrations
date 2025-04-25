# Go Migrations

**A database migrations tool & library**. It's targeted for Go projects, but it can be used as a 
standalone tool for any use case. It gives **flexibility** by allowing you to **organize and run 
database schema changes from Go files**.
So you are free to put any functionality you want in 
these migrations files, even if they are not strictly related to database schema management.  

## Use case, features, usage  
  
_**TLDR**_: read **README** file from **_examples** folder  

"**Go migrations**" allows users to **run Go functions sequentially** and it "remembers" the functions
that were ran so you can go back to a specific Go function or continue with new ones.  
It is mainly targeted for **database schema management**, but it **can be used for running basic, 
sequential workflows**.  
  
Each **migration** (or a step in a sequential workflow) **is a Go file** which must include a struct 
that implements the generic "Migration" interface. So the struct must define the implementation 
for the Version(), Up() and Down() methods.  
- **Version()** must return the migration identifier. If the migration file was generated 
  automatically from this tool (see examples README how to play with the tool), this method 
  should not be changed.  
- **Up()** must include the logic for making some changes on the database schema like adding a new 
  column or a new table.  
- **Down()** must include the logic to revert the changes done by Up()  
  
The project does not include pre-built binaries so you will have to prepare a main entrypoint 
file and build a binary on your own. **To make this easy, there are a few examples which you can 
use, in the _examples directory**.  
**Build tags** for storage integrations: **mysql** (works with mariadb also), **mongo**, 
**postgres** (more will be added)
  
## Recommendations & hints  

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
in the SQL database scenarios, some features like `LOCK TABLES`, if used in the migration files, 
may conflict with the migrations repository queries. So either you make sure you are doing some 
cleanup in Up(), Down() migration functions,if needed, or, use different db handles, connections.
The examples from _examples folder have been implemented with these aspects in mind.