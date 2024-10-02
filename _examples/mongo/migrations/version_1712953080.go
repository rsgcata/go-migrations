package migrations

import (
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type Migration1712953080 struct {
	Client *mongo.Client
	DbName string
	Ctx    context.Context
}

func (migration *Migration1712953080) Version() uint64 {
	return 1712953080
}

func (migration *Migration1712953080) Up() error {
	collection := migration.Client.Database(migration.DbName).Collection("users")
	_, err := collection.UpdateMany(
		migration.Ctx, bson.D{}, bson.D{
			{"$rename", bson.D{{"phone", "phoneNumber"}}},
		},
	)
	return err
}

func (migration *Migration1712953080) Down() error {
	collection := migration.Client.Database(migration.DbName).Collection("users")
	_, err := collection.UpdateMany(
		migration.Ctx, bson.D{}, bson.D{
			{"$rename", bson.D{{"phoneNumber", "phone"}}},
		},
	)
	return err
}
