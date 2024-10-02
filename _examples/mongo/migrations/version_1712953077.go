package migrations

import (
	"context"
	"go.mongodb.org/mongo-driver/mongo"
)

type Migration1712953077 struct {
	Client *mongo.Client
	DbName string
	Ctx    context.Context
}

func (migration *Migration1712953077) Version() uint64 {
	return 1712953077
}

type user struct {
	Email    string `bson:"email"`
	Phone    string `bson:"phone"`
	FullName string `bson:"fullName"`
}

func (migration *Migration1712953077) Up() error {
	var users []interface{}

	for _, u := range []user{
		{"test@test12345.com", "123456", "John Doe"},
		{"test@test123456.com", "123456", "Jane Doe"},
		{"test@test1234567.com", "123456", "Clark Kent"},
		{"test@test12345678.com", "123456", "Mia Khan"},
		{"test@test123456789.com", "123456", "Alberta Buz"},
	} {
		users = append(users, u)
	}

	collection := migration.Client.Database(migration.DbName).Collection("users")
	_, err := collection.InsertMany(migration.Ctx, users)
	return err
}

func (migration *Migration1712953077) Down() error {
	collection := migration.Client.Database(migration.DbName).Collection("users")
	return collection.Drop(migration.Ctx)
}
