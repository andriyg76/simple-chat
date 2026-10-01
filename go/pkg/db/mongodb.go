package db

import (
	"context"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/agushuley/simple-chat/go/pkg/errs"
)

type MongoDB struct {
	client   *mongo.Client
	database *mongo.Database
}

func NewMongoDB(uri, dbName string) (*MongoDB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	return &MongoDB{
		client:   client,
		database: client.Database(dbName),
	}, nil
}

func (m *MongoDB) Collection(name string) *mongo.Collection {
	return m.database.Collection(name)
}

// legacyCollections are collections superseded by schema changes and no
// longer read or written by any code path — see the "Deploy note" entries
// in CLAUDE.md for what replaced each one.
var legacyCollections = []string{"messages", "conversation_checkpoints", "conversation_states"}

// DropLegacyCollections removes any of legacyCollections still present in
// the database, returning the names actually dropped. Safe to call on every
// startup — dropping a collection that doesn't exist is a no-op, so once a
// deployment is caught up this is just one cheap ListCollectionNames call.
func (m *MongoDB) DropLegacyCollections(ctx context.Context) ([]string, error) {
	existing, err := m.database.ListCollectionNames(ctx, bson.D{})
	if err != nil {
		return nil, errs.Wrap(err)
	}
	present := make(map[string]bool, len(existing))
	for _, n := range existing {
		present[n] = true
	}

	var dropped []string
	for _, name := range legacyCollections {
		if !present[name] {
			continue
		}
		if err := m.database.Collection(name).Drop(ctx); err != nil {
			return dropped, errs.Wrap(err)
		}
		dropped = append(dropped, name)
	}
	return dropped, nil
}

func (m *MongoDB) Disconnect(ctx context.Context) error {
	return m.client.Disconnect(ctx)
}

// DumpAll reads every document from every collection in the database,
// keyed by collection name. Intended for small-scale admin/debug exports —
// it loads each collection fully into memory.
func (m *MongoDB) DumpAll(ctx context.Context) (map[string][]bson.M, error) {
	names, err := m.database.ListCollectionNames(ctx, bson.D{})
	if err != nil {
		return nil, errs.Wrap(err)
	}
	sort.Strings(names)

	result := make(map[string][]bson.M, len(names))
	for _, name := range names {
		cur, err := m.database.Collection(name).Find(ctx, bson.D{})
		if err != nil {
			return nil, errs.Wrap(err)
		}
		docs := []bson.M{}
		if err := cur.All(ctx, &docs); err != nil {
			return nil, errs.Wrap(err)
		}
		result[name] = docs
	}
	return result, nil
}
