package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.etcd.io/bbolt"

	"tg-tui/internal/domains"
)

var (
	bucketChats    = []byte("chats")
	bucketMessages = []byte("messages")
	keyChatList    = []byte("all")
)

type Cache struct {
	db *bbolt.DB
}

func OpenCache(path string) (*Cache, error) {
	db, err := bbolt.Open(path, 0o600, &bbolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "timeout") {
			return nil, fmt.Errorf("open cache: timed out waiting for database lock at %s; another tg-tui instance may still be running", path)
		}
		return nil, fmt.Errorf("open cache: %w", err)
	}

	if err := db.Update(func(tx *bbolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(bucketChats); err != nil {
			return err
		}

		if _, err := tx.CreateBucketIfNotExists(bucketMessages); err != nil {
			return err
		}

		return nil
	}); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("init cache buckets: %w", err)
	}

	return &Cache{db: db}, nil
}

func (c *Cache) SaveChats(_ context.Context, chats []domains.ChatSummary) error {
	return c.db.Update(func(tx *bbolt.Tx) error {
		encoded, err := json.Marshal(chats)
		if err != nil {
			return fmt.Errorf("encode chats: %w", err)
		}

		return tx.Bucket(bucketChats).Put(keyChatList, encoded)
	})
}

func (c *Cache) LoadChats(_ context.Context) ([]domains.ChatSummary, error) {
	var chats []domains.ChatSummary

	err := c.db.View(func(tx *bbolt.Tx) error {
		data := tx.Bucket(bucketChats).Get(keyChatList)
		if len(data) == 0 {
			return nil
		}

		if err := json.Unmarshal(data, &chats); err != nil {
			return fmt.Errorf("decode chats: %w", err)
		}

		return nil
	})

	return chats, err
}

func (c *Cache) SaveMessages(_ context.Context, chatID domains.ChatID, messages []domains.Message) error {
	return c.db.Update(func(tx *bbolt.Tx) error {
		encoded, err := json.Marshal(messages)
		if err != nil {
			return fmt.Errorf("encode messages: %w", err)
		}

		return tx.Bucket(bucketMessages).Put(messageKey(chatID), encoded)
	})
}

func (c *Cache) LoadMessages(_ context.Context, chatID domains.ChatID) ([]domains.Message, error) {
	var messages []domains.Message

	err := c.db.View(func(tx *bbolt.Tx) error {
		data := tx.Bucket(bucketMessages).Get(messageKey(chatID))
		if len(data) == 0 {
			return nil
		}

		if err := json.Unmarshal(data, &messages); err != nil {
			return fmt.Errorf("decode messages: %w", err)
		}

		return nil
	})

	return messages, err
}

func (c *Cache) Close() error {
	return c.db.Close()
}

func messageKey(chatID domains.ChatID) []byte {
	return []byte(strconv.FormatInt(int64(chatID), 10))
}
