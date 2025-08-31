package main

import (
	"context"
	"time"

	"github.com/go-telegram/bot/models"
)

type TypingChange struct {
	ChatID    int64
	MessageID int
	Action    models.ChatAction
}

type typingHandlerType struct {
	ch        chan TypingChange
	typingIDs map[int]TypingChange // map[MessageID]TypingChange
}

var typingHandler typingHandlerType

func (t *typingHandlerType) ChangeTypingStatus(chatID int64, messageID int, action models.ChatAction) {
	t.ch <- TypingChange{
		ChatID:    chatID,
		MessageID: messageID,
		Action:    action,
	}
}

func (t *typingHandlerType) Process(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case typingChange := <-t.ch:
			delete(t.typingIDs, typingChange.MessageID)

			if typingChange.Action != "" {
				t.typingIDs[typingChange.MessageID] = TypingChange{
					ChatID:    typingChange.ChatID,
					MessageID: typingChange.MessageID,
					Action:    typingChange.Action,
				}
				sendChatAction(ctx, typingChange.ChatID, typingChange.Action)
			}
		case <-time.After(4 * time.Second):
			var sendTypingTo []TypingChange
			for _, typingChange := range t.typingIDs {
				alredyInSendTypingTo := false
				for _, v := range sendTypingTo {
					if v.ChatID == typingChange.ChatID {
						alredyInSendTypingTo = true
						break
					}
				}
				if !alredyInSendTypingTo {
					sendTypingTo = append(sendTypingTo, typingChange)
				}
			}
			for _, typingChange := range sendTypingTo {
				sendChatAction(ctx, typingChange.ChatID, typingChange.Action)
			}
		}
	}
}

func (c *typingHandlerType) Start(ctx context.Context) {
	c.ch = make(chan TypingChange)
	c.typingIDs = make(map[int]TypingChange)
	go c.Process(ctx)
}
