package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot/models"
	"google.golang.org/genai"
)

var modelName = "gemini-3-pro-image-preview"

type ImageFilesDataType struct {
	Data     []byte
	Filename string
	MimeType string
}

type cmdHandlerType struct {
	cmdMsg            *models.Message
	expectImageFromID int64
	expectImageChan   chan ImageFilesDataType
}

func (c *cmdHandlerType) reply(ctx context.Context, text string) (replyMsg *models.Message, err error) {
	if c.cmdMsg == nil {
		return sendMessage(ctx, c.cmdMsg.Chat.ID, text)
	}
	return sendReplyToMessage(ctx, c.cmdMsg, text)
}

func (c *cmdHandlerType) ImagenResultProcess(ctx context.Context, response *genai.GenerateContentResponse, argsPresent []string, n int, prompt string) {
	typingHandler.ChangeTypingStatus(c.cmdMsg.Chat.ID, c.cmdMsg.ID, models.ChatActionUploadPhoto)
	defer func() {
		typingHandler.ChangeTypingStatus(c.cmdMsg.Chat.ID, c.cmdMsg.ID, "")
	}()

	if response.PromptFeedback != nil {
		_, _ = c.reply(ctx, errorStr+": "+response.PromptFeedback.BlockReasonMessage)
		return
	}

	if len(response.Candidates) == 0 {
		fmt.Println("    no candidates received")
		_, _ = c.reply(ctx, errorStr+": no images generated")
		return
	}

	// Extract image data from the response
	var imgs [][]byte
	for _, candidate := range response.Candidates {
		if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
			_, _ = c.reply(ctx, errorStr+": "+string(candidate.FinishReason))
			return
		}
		fmt.Printf("    candidate %v:\n", candidate)
		fmt.Printf("    parts: %v:\n", candidate.Content.Parts)
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				_, _ = c.reply(ctx, part.Text)
			} else if part.InlineData != nil {
				imgs = append(imgs, part.InlineData.Data)
			} else {
				fmt.Printf("    no image data found in part %v\n", part)
			}
		}
	}

	if len(imgs) == 0 {
		fmt.Println("    no images generated")
		_, _ = c.reply(ctx, errorStr+": no images generated")
		return
	}

	// Create a description for the image

	description := "💭 " + prompt
	// if len(argsPresent) > 0 {
	// 	argsDesc := ""
	// 	for _, arg := range argsPresent {
	// 		if argsDesc != "" {
	// 			argsDesc += " "
	// 		}

	// 		switch arg {
	// 		case "size":
	// 			argsDesc += "Size: " + size
	// 		case "background":
	// 			argsDesc += "Background: " + background
	// 		case "quality":
	// 			argsDesc += "Quality: " + quality
	// 		}
	// 	}
	// 	description += "\n🖼️ " + argsDesc
	// }

	fmt.Println("    uploading images...")
	_, err := uploadImages(ctx, c.cmdMsg, description, imgs)

	if err != nil {
		fmt.Println("    upload error:", err)
		_, _ = c.reply(ctx, errorStr+": "+err.Error())
		return
	}
	fmt.Println("    images uploaded successfully")
}

func (c *cmdHandlerType) ImagenEdit(ctx context.Context, argsPresent []string, n int, prompt string) {
	c.expectImageChan = make(chan ImageFilesDataType)

	if c.cmdMsg.ReplyToMessage != nil && (c.cmdMsg.ReplyToMessage.Document != nil || len(c.cmdMsg.ReplyToMessage.Photo) > 0) {
		c.expectImageFromID = int64(c.cmdMsg.ReplyToMessage.From.ID)
		go handleImageMessage(ctx, c.cmdMsg.ReplyToMessage)
	} else {
		c.expectImageFromID = c.cmdMsg.From.ID
		fmt.Println("    waiting for image data...")
		_, _ = c.reply(ctx, "🩻 Please post the image file(s) to process.")
	}

	var err error
	var imgs []ImageFilesDataType
	select {
	case img := <-c.expectImageChan:
		if len(img.Data) == 0 {
			break
		}

		imgs = append(imgs, img)

		// Wait for more images or timeout
	waitForMultipleImages:
		for {
			select {
			case img := <-c.expectImageChan:
				imgs = append(imgs, img)
			case <-ctx.Done():
				err = fmt.Errorf("context done")
				break waitForMultipleImages
			case <-time.NewTimer(1 * time.Second).C:
				break waitForMultipleImages
			}
		}
	case <-ctx.Done():
		err = fmt.Errorf("context done")
	case <-time.NewTimer(3 * time.Minute).C:
		err = fmt.Errorf("waiting for image data timeout")
	}
	close(c.expectImageChan)
	c.expectImageChan = nil

	if err == nil && len(imgs) == 0 {
		fmt.Println("    canceled")
		return
	}

	if err != nil {
		fmt.Println("    error:", err)
		_, _ = c.reply(ctx, errorStr+": "+err.Error())
		return
	}

	fmt.Println("    got", len(imgs), "images")

	typingHandler.ChangeTypingStatus(c.cmdMsg.Chat.ID, c.cmdMsg.ID, models.ChatActionTyping)

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: params.GeminiAPIKey,
	})
	if err != nil {
		log.Fatal(err)
	}

	parts := []*genai.Part{
		genai.NewPartFromText(prompt),
	}

	for _, img := range imgs {
		parts = append(parts, &genai.Part{
			InlineData: &genai.Blob{
				MIMEType: img.MimeType,
				Data:     img.Data,
			},
		})
	}

	contents := []*genai.Content{
		genai.NewContentFromParts(parts, genai.RoleUser),
	}

	fmt.Println("    sending edit request...")
	res, err := client.Models.GenerateContent(ctx, modelName, contents, &genai.GenerateContentConfig{
		CandidateCount: int32(n),
	})

	if err != nil {
		fmt.Println("    edit error:", err)
		_, _ = c.reply(ctx, errorStr+": "+err.Error())
		typingHandler.ChangeTypingStatus(c.cmdMsg.Chat.ID, c.cmdMsg.ID, "")
		return
	}

	c.ImagenResultProcess(ctx, res, argsPresent, n, prompt)
}

func (c *cmdHandlerType) ImagenGenerate(ctx context.Context, argsPresent []string, n int, prompt string) {
	typingHandler.ChangeTypingStatus(c.cmdMsg.Chat.ID, c.cmdMsg.ID, models.ChatActionTyping)

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: params.GeminiAPIKey,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("    sending generate request...")
	res, err := client.Models.GenerateContent(ctx, modelName, genai.Text(prompt), &genai.GenerateContentConfig{
		CandidateCount: int32(n),
	})

	if err != nil {
		fmt.Println("    generate error:", err)
		_, _ = c.reply(ctx, errorStr+": "+err.Error())
		typingHandler.ChangeTypingStatus(c.cmdMsg.Chat.ID, c.cmdMsg.ID, "")
		return
	}

	c.ImagenResultProcess(ctx, res, argsPresent, n, prompt)
}

func (c *cmdHandlerType) Imagen(ctx context.Context) {
	// Parse command arguments
	var argsPresent []string
	isEdit := false
	n := 1
	promptParts := []string{}

	// Split text into words
	words := strings.Fields(c.cmdMsg.Text)
	i := 0

	// Parse arguments
	for i < len(words) {
		word := words[i]

		if strings.HasPrefix(word, "-") {
			argName := strings.TrimPrefix(word, "-")

			switch argName {
			case "edit":
				isEdit = true
			case "n":
				if i+1 >= len(words) || strings.HasPrefix(words[i+1], "-") {
					fmt.Println("\tMissing value for flag:", argName)
					_, _ = c.reply(ctx, errorStr+": Missing value for flag: "+argName)
					return
				}

				argsPresent = append(argsPresent, argName)

				value := words[i+1]
				i++ // Skip the next word as we've processed it

				switch argName {
				case "n":
					var err error
					n, err = strconv.Atoi(value)
					if err != nil {
						fmt.Println("\tInvalid value for n:", value)
						_, _ = c.reply(ctx, errorStr+": Invalid value for n: "+value)
						return
					}
				}
			}
		} else {
			// Not a flag, add to prompt
			promptParts = append(promptParts, word)
		}

		i++
	}

	// Combine prompt parts into the final prompt
	prompt := strings.Join(promptParts, " ")
	prompt = strings.TrimSpace(prompt)

	if prompt == "" {
		fmt.Println("\tNo prompt provided")
		_, _ = c.reply(ctx, errorStr+": No prompt provided")
		return
	}

	if c.cmdMsg.ReplyToMessage != nil && (c.cmdMsg.ReplyToMessage.Document != nil || len(c.cmdMsg.ReplyToMessage.Photo) > 0) {
		isEdit = true
	}

	fmt.Println("    parsed args: n:", n, "edit:", isEdit, "prompt:", prompt)

	if isEdit {
		c.ImagenEdit(ctx, argsPresent, n, prompt)
		return
	}
	c.ImagenGenerate(ctx, argsPresent, n, prompt)
}

func (c *cmdHandlerType) Cancel(ctx context.Context) {
	// Searching for the handler that is expecting image data.
	var cmdHandler *cmdHandlerType
	cmdHandlersMutex.Lock()
	defer cmdHandlersMutex.Unlock()
	for i, h := range cmdHandlers {
		if h.expectImageFromID == c.cmdMsg.From.ID && h.expectImageChan != nil {
			cmdHandler = cmdHandlers[i]
			break
		}
	}

	if cmdHandler == nil {
		fmt.Println("  no handler waiting for image data")
		_, _ = c.reply(ctx, errorStr+": not waiting for image data")
		return
	}

	fmt.Println("  canceling waiting for image data")
	_, _ = c.reply(ctx, "❌ Canceling waiting for image data")
	cmdHandler.expectImageFromID = 0
	cmdHandler.expectImageChan <- ImageFilesDataType{}
}

func (c *cmdHandlerType) Help(ctx context.Context, cmdChar string) {
	_, _ = sendReplyToMessage(ctx, c.cmdMsg, "🤖 Imagen Telegram Bot\n\n"+
		"Available commands:\n\n"+
		cmdChar+"imagen (args) [prompt]\n"+
		"  args can be:\n"+
		"    -edit: toggles edit mode (auto enabled if you reply to an image)\n"+
		"    -n 1: generate n output images\n"+
		// "    -size 1024x1024\n"+
		// "    -background transparent (default is opaque)\n"+
		// "    -quality auto\n"+
		cmdChar+"imagencancel - cancel waiting for images\n\n"+
		cmdChar+"imagenhelp - show this help\n\n"+
		"For more information see https://github.com/nonoo/imagen-gemini-telegram-bot")
}
