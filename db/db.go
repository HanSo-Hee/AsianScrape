/*
 * Asian Drama Scraper Bot
 * Author: Abhishek (MrAbhi2k3)
 * GitHub: https://github.com/mrabhi2k3
 */

package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"time"

	_ "github.com/glebarez/go-sqlite"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Database struct {
	useMongo     bool
	mongoClient  *mongo.Client
	mongoDB      *mongo.Database
	postedCol    *mongo.Collection
	fileStoreCol *mongo.Collection
	peerStoreCol *mongo.Collection
	sqliteDB     *sql.DB
}

var Global *Database

func Init(mongoSRV string) {
	d := &Database{}
	if mongoSRV != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoSRV))
		if err == nil {
			err = client.Ping(ctx, nil)
			if err == nil {
				d.useMongo = true
				d.mongoClient = client
				d.mongoDB = client.Database("asianscraper")
				d.postedCol = d.mongoDB.Collection("posted_items")
				d.fileStoreCol = d.mongoDB.Collection("file_store")
				d.peerStoreCol = d.mongoDB.Collection("peers")
				log.Println("Connected to MongoDB successfully")
			}
		}
	}

	if !d.useMongo {
		log.Println("Using SQLite fallback (dramas.db)")
		db, err := sql.Open("sqlite", "dramas.db")
		if err != nil {
			log.Fatalf("Failed to open SQLite db: %v", err)
		}
		d.sqliteDB = db
		d.initSQLite()
	}

	Global = d
}

func (d *Database) Close() {
	if d.useMongo && d.mongoClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = d.mongoClient.Disconnect(ctx)
	} else if d.sqliteDB != nil {
		_ = d.sqliteDB.Close()
	}
}

func (d *Database) initSQLite() {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS posted_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			item_url TEXT UNIQUE,
			title TEXT,
			posted_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS file_store (
			id TEXT PRIMARY KEY,
			title TEXT,
			qualities TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS peers (
			id INTEGER PRIMARY KEY,
			access_hash INTEGER,
			type TEXT
		)`,
	}
	for _, q := range queries {
		_, err := d.sqliteDB.Exec(q)
		if err != nil {
			log.Fatalf("Failed to initialize SQLite: %v", err)
		}
	}
}

func (d *Database) IsPosted(itemURL string) bool {
	if d.useMongo {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		count, err := d.postedCol.CountDocuments(ctx, bson.M{"item_url": itemURL})
		if err != nil {
			return false
		}
		return count > 0
	} else {
		var exists int
		err := d.sqliteDB.QueryRow("SELECT 1 FROM posted_items WHERE item_url = ?", itemURL).Scan(&exists)
		return err == nil
	}
}

func (d *Database) MarkPosted(itemURL string, title string) {
	if d.useMongo {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		opts := options.Update().SetUpsert(true)
		_, err := d.postedCol.UpdateOne(ctx, bson.M{"item_url": itemURL}, bson.M{"$set": bson.M{"item_url": itemURL, "title": title}}, opts)
		if err != nil {
			log.Printf("DB Error MarkPosted: %v", err)
		}
	} else {
		_, err := d.sqliteDB.Exec("INSERT OR REPLACE INTO posted_items (item_url, title) VALUES (?, ?)", itemURL, title)
		if err != nil {
			log.Printf("DB Error MarkPosted: %v", err)
		}
	}
}

func (d *Database) SaveFileQualities(fileID string, title string, qualities map[string]interface{}) {
	if d.useMongo {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		opts := options.Update().SetUpsert(true)
		_, err := d.fileStoreCol.UpdateOne(ctx, bson.M{"_id": fileID}, bson.M{"$set": bson.M{"title": title, "qualities": qualities}}, opts)
		if err != nil {
			log.Printf("DB Error SaveFileQualities: %v", err)
		}
	} else {
		qualitiesBytes, _ := json.Marshal(qualities)
		_, err := d.sqliteDB.Exec("INSERT OR REPLACE INTO file_store (id, title, qualities) VALUES (?, ?, ?)", fileID, title, string(qualitiesBytes))
		if err != nil {
			log.Printf("DB Error SaveFileQualities: %v", err)
		}
	}
}

func (d *Database) GetFileQualities(fileID string) (string, map[string]interface{}, bool) {
	if d.useMongo {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var result bson.M
		err := d.fileStoreCol.FindOne(ctx, bson.M{"_id": fileID}).Decode(&result)
		if err != nil {
			return "", nil, false
		}
		title, _ := result["title"].(string)
		qualities := make(map[string]interface{})
		if qRaw, ok := result["qualities"]; ok {
			b, err := json.Marshal(qRaw)
			if err == nil {
				_ = json.Unmarshal(b, &qualities)
			}
		}
		return title, qualities, true
	} else {
		var title string
		var qualitiesStr string
		err := d.sqliteDB.QueryRow("SELECT title, qualities FROM file_store WHERE id = ?", fileID).Scan(&title, &qualitiesStr)
		if err != nil {
			return "", nil, false
		}
		var qualities map[string]interface{}
		_ = json.Unmarshal([]byte(qualitiesStr), &qualities)
		return title, qualities, true
	}
}

func (d *Database) SavePeer(peerID int64, accessHash int64, peerType string) {
	if d.useMongo {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		opts := options.Update().SetUpsert(true)
		_, err := d.peerStoreCol.UpdateOne(ctx, bson.M{"_id": peerID}, bson.M{"$set": bson.M{"access_hash": accessHash, "type": peerType}}, opts)
		if err != nil {
			log.Printf("DB Error SavePeer: %v", err)
		}
	} else {
		_, err := d.sqliteDB.Exec("INSERT OR REPLACE INTO peers (id, access_hash, type) VALUES (?, ?, ?)", peerID, accessHash, peerType)
		if err != nil {
			log.Printf("DB Error SavePeer: %v", err)
		}
	}
}

func (d *Database) GetPeer(peerID int64) (int64, string, bool) {
	if d.useMongo {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var result bson.M
		err := d.peerStoreCol.FindOne(ctx, bson.M{"_id": peerID}).Decode(&result)
		if err != nil {
			return 0, "", false
		}
		var accessHash int64
		if ah, ok := result["access_hash"].(int64); ok {
			accessHash = ah
		} else if ah, ok := result["access_hash"].(int32); ok {
			accessHash = int64(ah)
		} else if ah, ok := result["access_hash"].(float64); ok {
			accessHash = int64(ah)
		}
		peerType, _ := result["type"].(string)
		return accessHash, peerType, true
	} else {
		var accessHash int64
		var peerType string
		err := d.sqliteDB.QueryRow("SELECT access_hash, type FROM peers WHERE id = ?", peerID).Scan(&accessHash, &peerType)
		if err != nil {
			return 0, "", false
		}
		return accessHash, peerType, true
	}
}
