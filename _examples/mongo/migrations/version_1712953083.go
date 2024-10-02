package migrations

import (
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"strings"
)

const FullNameSplitLock = "full-name-split-lock"

type userWithNewPhone struct {
	Email    string `bson:"email"`
	Phone    string `bson:"phoneNumber"`
	FullName string `bson:"fullName"`
}

type userWithFullNameSplit struct {
	Email     string `bson:"email"`
	Phone     string `bson:"phoneNumber"`
	FirstName string `bson:"firstName"`
	LastName  string `bson:"lastName"`
}

type Migration1712953083 struct {
	Client *mongo.Client
	DbName string
	Ctx    context.Context
}

func (migration *Migration1712953083) Version() uint64 {
	return 1712953083
}

type runChange func(session mongo.Session) error

func (migration *Migration1712953083) lockAndRunChange(run runChange) error {
	session, err := migration.Client.StartSession()

	if err != nil {
		return err
	}

	err = session.StartTransaction()

	if err != nil {
		return err
	}

	locksCollection := session.Client().Database(migration.DbName).Collection("locks")

	// Obtain collection lock for update
	locksCollection.FindOneAndUpdate(
		migration.Ctx,
		bson.D{{"lockName", FullNameSplitLock}},
		bson.D{
			{"$set", bson.D{{"randVal", primitive.NewObjectID()}}},
		},
	)

	err = run(session)
	_, _ = locksCollection.DeleteOne(migration.Ctx, bson.D{{"lockName", FullNameSplitLock}})

	if err != nil {
		_ = session.AbortTransaction(migration.Ctx)
		return err
	}

	if err = session.CommitTransaction(migration.Ctx); err != nil {
		_ = session.AbortTransaction(migration.Ctx)
		return err
	}

	return nil
}

func (migration *Migration1712953083) Up() error {
	return migration.lockAndRunChange(
		func(session mongo.Session) error {
			usersCollection := session.Client().Database(migration.DbName).Collection("users")
			usersCursor, err := usersCollection.Find(
				migration.Ctx,
				bson.D{},
			)

			if usersCursor == nil {
				return nil
			}

			var results []userWithNewPhone
			err = usersCursor.All(migration.Ctx, &results)

			if err != nil {
				return err
			}

			for _, userToChange := range results {
				nameSplit := strings.Split(userToChange.FullName, " ")
				changedUser := userWithFullNameSplit{
					Email:     userToChange.Email,
					Phone:     userToChange.Phone,
					FirstName: nameSplit[0],
					LastName:  nameSplit[1],
				}
				_, err = usersCollection.ReplaceOne(
					migration.Ctx,
					bson.D{{"email", userToChange.Email}},
					changedUser,
				)

				if err != nil {
					return err
				}
			}

			return nil
		},
	)
}

func (migration *Migration1712953083) Down() error {
	return migration.lockAndRunChange(
		func(session mongo.Session) error {
			usersCollection := session.Client().Database(migration.DbName).Collection("users")
			usersCursor, err := usersCollection.Find(
				migration.Ctx,
				bson.D{},
			)

			if usersCursor == nil {
				return nil
			}

			var results []userWithFullNameSplit
			err = usersCursor.All(migration.Ctx, &results)

			if err != nil {
				return err
			}

			for _, userToChange := range results {
				fullName := userToChange.FirstName + " " + userToChange.LastName
				changedUser := user{
					Email:    userToChange.Email,
					Phone:    userToChange.Phone,
					FullName: fullName,
				}
				_, err = usersCollection.ReplaceOne(
					migration.Ctx,
					bson.D{{"email", userToChange.Email}},
					changedUser,
				)

				if err != nil {
					return err
				}
			}

			return nil
		},
	)
}
