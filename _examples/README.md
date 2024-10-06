## Examples  
  
Each folder integrates with a storage type (repository) the library supports  
To play with these examples follow the bellow steps:  
  
1. Change directory to project root
2. Start the containers: ``docker compose up -d``
3. SSH into lib-dev container: ``docker exec -it lib-dev bash``
4. Change directory to one of the available storage integrations: ``cd ./_examples/mysql``
5. Build the binary: ``go build -tags mysql -o ./bin/migrate``
6. Get helpful info from the migrate binary: ``./bin/migrate help``
7. Run one migration Up() from the migrate binary: ``./bin/migrate up``
8. Run 3 migrations Up() from the migrate binary: ``./bin/migrate up 3``
9. Run all migrations Up() from the migrate binary: ``./bin/migrate up all``