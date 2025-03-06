package database

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/generative-ai-go/genai"
	"github.com/integems/report-agent/config"
	"github.com/redis/go-redis/v9"
)

func NewRedisConnection() *redis.Client {
	redisPort := config.GetEnv("REDIS_PORT", "6379")
	redisHost := config.GetEnv("REDIS_HOST", "127.0.0.1")
	redisAddr := fmt.Sprintf("%v:%v", redisHost, redisPort)
	client := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "", // No password set
		DB:       0,  // Use default DB
	})
	return client
}

type RedisSessionManager struct {
	client *redis.Client
	ctx    context.Context
}

type Message struct {
	Role        string    `json:"role"` // "user" or "ai"
	Content     any       `json:"content"`
	CreatedAt   time.Time `json:"createdAt"` // Timestamp
	ContentType string    `json:"contentType"`
}

// NewRedisSessionManager initializes a Redis client
func NewRedisSessionManager() *RedisSessionManager {
	client := NewRedisConnection()
	return &RedisSessionManager{
		client: client,
		ctx:    context.Background(),
	}
}

// GetSession retrieves a session for the given videoID
func (r *RedisSessionManager) GetSessionHistory(videoID string) ([]*genai.Content, error) {

	key := fmt.Sprintf("messages:%s", videoID)
	var history []*genai.Content = []*genai.Content{{
		Parts: []genai.Part{
			genai.Text("Answer questions, fetch answers from the internet, and answer questions relating to the files if given. Be interactive."),
		},
		Role: "user",
	}}

	messagesData, err := r.client.LRange(r.ctx, key, 0, -1).Result()
	if err != nil {
		// log.Println(err)
		return nil, fmt.Errorf("failed to retrieve messages: %w", err)
	}

	for _, data := range messagesData {
		var message Message
		if err := json.Unmarshal([]byte(data), &message); err != nil {
			log.Printf("Failed to unmarshal message: %v", err)
			continue
		}
		contentType := message.ContentType
		text := message.Content.(string)

		if contentType == "file" {
			content := genai.Content{Parts: []genai.Part{
				genai.FileData{URI: text},
			}, Role: message.Role}

			history = append(history, &content)

		} else {
			content := genai.Content{Parts: []genai.Part{
				genai.Text(text),
			}, Role: message.Role}
			history = append(history, &content)

		}
	}

	return history, nil
}

// SaveMessage saves a chat message to Redis
func (r *RedisSessionManager) SaveMessage(videoID string, role string, content any, contentType string) error {

	message := Message{
		Role:        role,
		Content:     content,
		CreatedAt:   time.Now(),
		ContentType: contentType,
	}

	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	key := fmt.Sprintf("messages:%s", videoID)
	if err := r.client.RPush(r.ctx, key, data).Err(); err != nil {
		return fmt.Errorf("failed to save message to Redis: %w", err)
	}

	// Set expiration for the list
	if err := r.client.Expire(r.ctx, key, 48*time.Hour).Err(); err != nil {
		return fmt.Errorf("failed to set expiration for message list: %w", err)
	}

	return nil
}

// GetMessages retrieves all chat messages for a given videoId
func (r *RedisSessionManager) GetMessages(sessionId string) ([]Message, error) {
	key := fmt.Sprintf("messages:%s", sessionId)
	messagesData, err := r.client.LRange(r.ctx, key, 0, -1).Result()
	if err != nil {
		// log.Println(err)
		return nil, fmt.Errorf("failed to retrieve messages: %w", err)
	}
	// log.Println(messagesData)
	var messages []Message
	for _, data := range messagesData {
		var message Message
		if err := json.Unmarshal([]byte(data), &message); err != nil {
			log.Printf("Failed to unmarshal message: %v", err)
			continue
		}
		messages = append(messages, message)
	}
	return messages, nil
}
