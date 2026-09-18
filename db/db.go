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
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	_ "github.com/glebarez/go-sqlite"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ScheduledDeletionTask struct {
	ID       string    `bson:"_id,omitempty" json:"id"`
	ChatID   int64     `bson:"chat_id" json:"chat_id"`
	MsgIDs   []int32   `bson:"msg_ids" json:"msg_ids"`
	DeleteAt time.Time `bson:"delete_at" json:"delete_at"`
}

type Database struct {
	useMongo     bool
	mongoClient  *mongo.Client
	mongoDB      *mongo.Database
	postedCol    *mongo.Collection
	fileStoreCol *mongo.Collection
	peerStoreCol *mongo.Collection
	deletionCol  *mongo.Collection
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
				d.deletionCol = d.mongoDB.Collection("scheduled_deletions")
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
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS file_store (
			id TEXT PRIMARY KEY,
			title TEXT,
			qualities TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS peers (
			id INTEGER PRIMARY KEY,
			access_hash INTEGER,
			username TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS scheduled_deletions (
			id TEXT PRIMARY KEY,
			chat_id INTEGER,
			msg_ids TEXT,
			delete_at DATETIME
		);`,
	}

	for _, q := range queries {
		_, err := d.sqliteDB.Exec(q)
		if err != nil {
			log.Fatalf("Failed to init SQLite schema: %v", err)
		}
	}
}

func (d *Database) IsPosted(itemURL string) bool {
	if d.useMongo {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		count, err := d.postedCol.CountDocuments(ctx, bson.M{"item_url": itemURL})
		return err == nil && count > 0
	} else {
		var id int
		err := d.sqliteDB.QueryRow("SELECT id FROM posted_items WHERE item_url = ?", itemURL).Scan(&id)
		return err == nil
	}
}

func (d *Database) MarkPosted(itemURL string, title string) {
	if d.useMongo {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		opts := options.Update().SetUpsert(true)
		_, err := d.postedCol.UpdateOne(ctx, bson.M{"item_url": itemURL}, bson.M{"$set": bson.M{"title": title}}, opts)
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

func normalizeID(fileID string) string {
	return regexp.MustCompile(`[^a-zA-Z0-9]+`).ReplaceAllString(strings.ToLower(fileID), "_")
}

func (d *Database) SaveFileQualities(fileID string, title string, qualities map[string]interface{}) {
	normKey := normalizeID(fileID)
	if d.useMongo {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		opts := options.Update().SetUpsert(true)
		_, err := d.fileStoreCol.UpdateOne(ctx, bson.M{"_id": normKey}, bson.M{"$set": bson.M{"title": title, "qualities": qualities}}, opts)
		if err != nil {
			log.Printf("DB Error SaveFileQualities: %v", err)
		}
	} else {
		qualitiesBytes, _ := json.Marshal(qualities)
		_, err := d.sqliteDB.Exec("INSERT OR REPLACE INTO file_store (id, title, qualities) VALUES (?, ?, ?)", normKey, title, string(qualitiesBytes))
		if err != nil {
			log.Printf("DB Error SaveFileQualities: %v", err)
		}
	}
}

func (d *Database) GetFileQualities(fileID string) (string, map[string]interface{}, bool) {
	keysToTry := []string{normalizeID(fileID), fileID, strings.ReplaceAll(fileID, "_", " ")}
	for _, key := range keysToTry {
		if d.useMongo {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			var result bson.M
			err := d.fileStoreCol.FindOne(ctx, bson.M{"_id": key}).Decode(&result)
			cancel()
			if err == nil {
				title, _ := result["title"].(string)
				qualities := make(map[string]interface{})
				if qRaw, ok := result["qualities"]; ok {
					b, err := json.Marshal(qRaw)
					if err == nil {
						_ = json.Unmarshal(b, &qualities)
					}
				}
				return title, qualities, true
			}
		} else {
			var title string
			var qualitiesStr string
			err := d.sqliteDB.QueryRow("SELECT title, qualities FROM file_store WHERE id = ?", key).Scan(&title, &qualitiesStr)
			if err == nil {
				var qualities map[string]interface{}
				_ = json.Unmarshal([]byte(qualitiesStr), &qualities)
				return title, qualities, true
			}
		}
	}
	return "", nil, false
}

func (d *Database) DeleteShow(fileID string) bool {
	normKey := normalizeID(fileID)
	keysToTry := []string{normKey, fileID, strings.ReplaceAll(fileID, "_", " ")}
	deleted := false

	if d.useMongo {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for _, k := range keysToTry {
			res, err := d.fileStoreCol.DeleteOne(ctx, bson.M{"_id": k})
			if err == nil && res.DeletedCount > 0 {
				deleted = true
			}
			_, _ = d.postedCol.DeleteMany(ctx, bson.M{"item_url": bson.M{"$regex": k, "$options": "i"}})
		}
	} else {
		for _, k := range keysToTry {
			res, err := d.sqliteDB.Exec("DELETE FROM file_store WHERE id = ?", k)
			if err == nil {
				if n, _ := res.RowsAffected(); n > 0 {
					deleted = true
				}
			}
			_, _ = d.sqliteDB.Exec("DELETE FROM posted_items WHERE item_url LIKE ? OR title LIKE ?", "%"+k+"%", "%"+k+"%")
		}
	}
	return deleted
}

func (d *Database) SaveScheduledDeletion(chatID int64, msgIDs []int32, deleteAt time.Time) string {
	id := fmt.Sprintf("%d_%d", chatID, time.Now().UnixNano())
	task := ScheduledDeletionTask{
		ID:       id,
		ChatID:   chatID,
		MsgIDs:   msgIDs,
		DeleteAt: deleteAt,
	}

	if d.useMongo {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = d.deletionCol.InsertOne(ctx, task)
	} else {
		msgIDsBytes, _ := json.Marshal(msgIDs)
		_, _ = d.sqliteDB.Exec("INSERT INTO scheduled_deletions (id, chat_id, msg_ids, delete_at) VALUES (?, ?, ?, ?)", id, chatID, string(msgIDsBytes), deleteAt)
	}
	return id
}

func (d *Database) GetPendingDeletions() []ScheduledDeletionTask {
	var tasks []ScheduledDeletionTask
	now := time.Now()

	if d.useMongo {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cursor, err := d.deletionCol.Find(ctx, bson.M{"delete_at": bson.M{"$lte": now}})
		if err == nil {
			_ = cursor.All(ctx, &tasks)
		}
	} else {
		rows, err := d.sqliteDB.Query("SELECT id, chat_id, msg_ids, delete_at FROM scheduled_deletions WHERE delete_at <= ?", now)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var t ScheduledDeletionTask
				var msgIDsStr string
				if err := rows.Scan(&t.ID, &t.ChatID, &msgIDsStr, &t.DeleteAt); err == nil {
					_ = json.Unmarshal([]byte(msgIDsStr), &t.MsgIDs)
					tasks = append(tasks, t)
				}
			}
		}
	}
	return tasks
}

func (d *Database) RemoveScheduledDeletion(id string) {
	if d.useMongo {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = d.deletionCol.DeleteOne(ctx, bson.M{"_id": id})
	} else {
		_, _ = d.sqliteDB.Exec("DELETE FROM scheduled_deletions WHERE id = ?", id)
	}
}

func (d *Database) SaveUser(userID int64, username string) {
	if d.useMongo {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		filter := bson.M{"_id": userID}
		update := bson.M{
			"$set": bson.M{
				"username":   username,
				"updated_at": time.Now(),
			},
			"$setOnInsert": bson.M{
				"created_at": time.Now(),
			},
		}
		opts := options.Update().SetUpsert(true)
		_, _ = d.peerStoreCol.UpdateOne(ctx, filter, update, opts)
	} else if d.sqliteDB != nil {
		_, _ = d.sqliteDB.Exec("INSERT OR REPLACE INTO peers (id, access_hash, username) VALUES (?, 0, ?)", userID, username)
	}
}

func (d *Database) GetAllUserIDs() []int64 {
	var userIDs []int64
	if d.useMongo {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cursor, err := d.peerStoreCol.Find(ctx, bson.M{})
		if err == nil {
			defer cursor.Close(ctx)
			for cursor.Next(ctx) {
				var u struct {
					ID int64 `bson:"_id"`
				}
				if err := cursor.Decode(&u); err == nil && u.ID > 0 {
					userIDs = append(userIDs, u.ID)
				}
			}
		}
	} else if d.sqliteDB != nil {
		rows, err := d.sqliteDB.Query("SELECT id FROM peers WHERE id > 0")
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var id int64
				if err := rows.Scan(&id); err == nil && id > 0 {
					userIDs = append(userIDs, id)
				}
			}
		}
	}
	return userIDs
}

func (d *Database) GetDBStats() (showsCount int64, postedCount int64, pendingDeletions int64, usersCount int64) {
	if d.useMongo {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if d.fileStoreCol != nil {
			showsCount, _ = d.fileStoreCol.CountDocuments(ctx, bson.M{})
		}
		if d.postedCol != nil {
			postedCount, _ = d.postedCol.CountDocuments(ctx, bson.M{})
		}
		if d.deletionCol != nil {
			pendingDeletions, _ = d.deletionCol.CountDocuments(ctx, bson.M{})
		}
		if d.peerStoreCol != nil {
			usersCount, _ = d.peerStoreCol.CountDocuments(ctx, bson.M{})
		}
	} else if d.sqliteDB != nil {
		_ = d.sqliteDB.QueryRow("SELECT COUNT(*) FROM file_store").Scan(&showsCount)
		_ = d.sqliteDB.QueryRow("SELECT COUNT(*) FROM posted_items").Scan(&postedCount)
		_ = d.sqliteDB.QueryRow("SELECT COUNT(*) FROM scheduled_deletions").Scan(&pendingDeletions)
		_ = d.sqliteDB.QueryRow("SELECT COUNT(*) FROM peers WHERE id > 0").Scan(&usersCount)
	}
	return
}


