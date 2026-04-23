package vector

import (
	"context"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/qdrant/go-client/qdrant"
)

type QdrantClient struct {
	client *qdrant.Client
}

func NewQdrantClient(url string) *QdrantClient {
	config := &qdrant.Config{Host: url}

	client, err := qdrant.NewClient(config)
	if err != nil {
		panic(err)
	}
	return &QdrantClient{
		client: client,
	}
}

func (q *QdrantClient) CreateWebsiteCollection() {
	err := q.client.CreateCollection(context.Background(), &qdrant.CreateCollection{
		CollectionName: "data",
		VectorsConfig: qdrant.NewVectorsConfigMap(map[string]*qdrant.VectorParams{
			"website": {
				Size:     1024,
				Distance: qdrant.Distance_Cosine,
			},
			"ad": {
				Size:     1024,
				Distance: qdrant.Distance_Cosine,
			},
		}),
	})

	if err != nil {
		panic(err)
	}
}

func websitePointID(url string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(url))
	return h.Sum64()
}

func (q *QdrantClient) AddWebsiteVectorToQdrant(url string, vector []float32) error {
	wait := true
	id := websitePointID(url)

	_, err := q.client.Upsert(context.Background(), &qdrant.UpsertPoints{
		CollectionName: "data",
		Wait:           &(wait),
		Points: []*qdrant.PointStruct{
			{
				Id: qdrant.NewIDNum(id),
				Vectors: qdrant.NewVectorsMap(map[string]*qdrant.Vector{
					"website": qdrant.NewVector(vector...),
				}),
				Payload: qdrant.NewValueMap(map[string]any{
					"url":       url,
					"timestamp": time.Now().Format(time.RFC3339),
				}),
			},
		},
	})
	return err
}

func (q *QdrantClient) AddAdVectorToQdrant(accountKey, campaignKey string, tags []string, vector []float32) error {
	wait := true
	id := websitePointID(fmt.Sprintf("%s:%s", accountKey, campaignKey))

	_, err := q.client.Upsert(context.Background(), &qdrant.UpsertPoints{
		CollectionName: "marketing_data",
		Wait:           &(wait),
		Points: []*qdrant.PointStruct{
			{
				Id: qdrant.NewIDNum(id),
				Vectors: qdrant.NewVectorsMap(map[string]*qdrant.Vector{
					"ad": qdrant.NewVector(vector...),
				}),
				Payload: qdrant.NewValueMap(map[string]any{
					"accountKey":  accountKey,
					"campaignKey": campaignKey,
					"tags":        tags,
					"timestamp":   time.Now().Format(time.RFC3339),
				}),
			},
		},
	})
	if err != nil {
		return err
	}
	return nil
}

func (q *QdrantClient) FindBestAdsForWebsite(websiteVector []float32) ([]*qdrant.ScoredPoint, error) {
	using := "ad"
	limit := uint64(3)

	res, err := q.client.Query(context.Background(), &qdrant.QueryPoints{
		CollectionName: "marketing_data",
		Query:          qdrant.NewQuery(websiteVector...),
		Using:          &using,
		Filter: &qdrant.Filter{
			Must: []*qdrant.Condition{
				qdrant.NewMatch("type", "ad"), // Ensure we only get ads back
			},
		},
		Limit: &limit,
	})

	if err != nil {
		return nil, err
	}

	return res, nil
}
