package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Состояния диалога
const (
	StateChoosing     = "choosing"
	StateTypingReply  = "typing_reply"
	StateTypingChoice = "typing_choice"
)

// UserState хранит состояние разговора с конкретным пользователем.
type UserState struct {
	State  string            `json:"state"`
	Data   map[string]string `json:"data"`
	Choice string            `json:"choice"`
}

// NewUserState создаёт пустое состояние пользователя.
func NewUserState() *UserState {
	return &UserState{
		State: StateChoosing,
		Data:  make(map[string]string),
	}
}

// factsToStr форматирует сохранённые факты, как в Python-версии.
func factsToStr(data map[string]string) string {
	if len(data) == 0 {
		return "\n\n"
	}
	var parts []string
	for k, v := range data {
		parts = append(parts, fmt.Sprintf("%s - %s", k, v))
	}
	return "\n" + strings.Join(parts, "\n") + "\n"
}

// HandleCommandStart реализует поведение /start.
func (u *UserState) HandleCommandStart() string {
	var sb strings.Builder
	sb.WriteString("Hi! My name is Doctor Botter.")
	if len(u.Data) > 0 {
		// Ключи уже хранятся в нижнем регистре, как в Python-коде
		var keys []string
		for k := range u.Data {
			keys = append(keys, k)
		}
		sb.WriteString(" You already told me your ")
		sb.WriteString(strings.Join(keys, ", "))
		sb.WriteString(". Why don't you tell me something more about yourself? Or change anything I already know.")
	} else {
		sb.WriteString(" I will hold a more complex conversation with you. Why don't you tell me something about yourself?")
	}
	u.State = StateChoosing
	return sb.String()
}

// HandleShowData реализует /show_data.
func (u *UserState) HandleShowData() string {
	return "This is what you already told me: " + factsToStr(u.Data)
}

// HandleText обрабатывает обычный текст (не команды).
// Возвращает текст ответа, нужно ли показать клавиатуру и завершён ли диалог (“Done”).
func (u *UserState) HandleText(text string) (reply string, withKeyboard bool, done bool) {
	// Фраза "Done" работает из любого состояния.
	if text == "Done" {
		u.Choice = ""
		reply = "I learned these facts about you: " + factsToStr(u.Data) + "Until next time!"
		u.State = ""
		return reply, false, true
	}

	switch u.State {
	case StateChoosing:
		return u.handleChoosing(text)
	case StateTypingChoice:
		return u.handleTypingChoice(text)
	case StateTypingReply:
		return u.handleTypingReply(text)
	default:
		// Если по какой-то причине нет состояния — считаем, что снова выбираем.
		u.State = StateChoosing
		return u.handleChoosing(text)
	}
}

func (u *UserState) handleChoosing(text string) (string, bool, bool) {
	switch text {
	case "Age", "Favourite colour", "Number of siblings":
		choice := strings.ToLower(text)
		u.Choice = choice
		if existing, ok := u.Data[choice]; ok {
			return fmt.Sprintf("Your %s? I already know the following about that: %s", choice, existing), false, false
		}
		u.State = StateTypingReply
		return fmt.Sprintf("Your %s? Yes, I would love to hear about that!", choice), false, false

	case "Something else...":
		u.State = StateTypingChoice
		return "Alright, please send me the category first, for example \"Most impressive skill\"", false, false

	default:
		// В оригинале такого случая нет – добавим мягкое напоминание.
		return "Please choose one of the options on the keyboard or type \"Done\".", true, false
	}
}

func (u *UserState) handleTypingChoice(text string) (string, bool, bool) {
	// Пользователь прислал название категории (кастомный вариант)
	choice := strings.ToLower(text)
	u.Choice = choice
	if existing, ok := u.Data[choice]; ok {
		u.State = StateTypingReply
		return fmt.Sprintf("Your %s? I already know the following about that: %s", choice, existing), false, false
	}
	u.State = StateTypingReply
	return fmt.Sprintf("Your %s? Yes, I would love to hear about that!", choice), false, false
}

func (u *UserState) handleTypingReply(text string) (string, bool, bool) {
	if u.Choice == "" {
		// На всякий случай – если вдруг что-то сломалось.
		u.State = StateChoosing
		return "I am not sure what category this belongs to. Please choose one of the options.", true, false
	}
	category := u.Choice
	value := strings.ToLower(text)
	u.Data[category] = value
	u.Choice = ""
	u.State = StateChoosing

	reply := "Neat! Just so you know, this is what you already told me:" +
		factsToStr(u.Data) +
		"You can tell me more, or change your opinion on something."
	return reply, true, false
}

// ---------- Хранилище (файловая "БД") ----------

type Storage struct {
	path string
	mu   sync.Mutex
}

type persistedData struct {
	Users map[int64]*UserState `json:"users"`
}

func NewStorage(path string) *Storage {
	return &Storage{path: path}
}

func (s *Storage) Load() (map[int64]*UserState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data := persistedData{
		Users: make(map[int64]*UserState),
	}

	b, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return data.Users, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, err
	}
	if data.Users == nil {
		data.Users = make(map[int64]*UserState)
	}
	return data.Users, nil
}

func (s *Storage) Save(users map[int64]*UserState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data := persistedData{Users: users}
	b, err := json.MarshalIndent(&data, "", "  ")
	if err != nil {
		return err
	}
	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmpPath, s.path)
}

// ---------- Обёртка бота ----------

type Bot struct {
	api     *tgbotapi.BotAPI
	storage *Storage

	mu    sync.Mutex
	users map[int64]*UserState
}

func NewBot(api *tgbotapi.BotAPI, storage *Storage) (*Bot, error) {
	users, err := storage.Load()
	if err != nil {
		return nil, err
	}
	return &Bot{
		api:     api,
		storage: storage,
		users:   users,
	}, nil
}

func (b *Bot) getUserState(userID int64) *UserState {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.users == nil {
		b.users = make(map[int64]*UserState)
	}
	us, ok := b.users[userID]
	if !ok || us == nil {
		us = NewUserState()
		b.users[userID] = us
	}
	return us
}

func mainKeyboard() tgbotapi.ReplyKeyboardMarkup {
	kb := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Age"),
			tgbotapi.NewKeyboardButton("Favourite colour"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Number of siblings"),
			tgbotapi.NewKeyboardButton("Something else..."),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Done"),
		),
	)
	kb.OneTimeKeyboard = true
	return kb
}

func (b *Bot) save() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.storage.Save(b.users); err != nil {
		log.Printf("error saving state: %v", err)
	}
}

func (b *Bot) Run() error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)
	for update := range updates {
		if update.Message == nil {
			continue
		}
		b.handleMessage(update.Message)
	}
	return nil
}

func (b *Bot) handleMessage(msg *tgbotapi.Message) {
	if msg.From == nil {
		return
	}
	userID := msg.From.ID
	chatID := msg.Chat.ID
	text := msg.Text

	userState := b.getUserState(userID)

	var reply string
	var withKeyboard bool
	var done bool
	var removeKeyboard bool

	if msg.IsCommand() {
		switch msg.Command() {
		case "start":
			reply = userState.HandleCommandStart()
			withKeyboard = true
		case "show_data":
			reply = userState.HandleShowData()
		default:
			reply = "Unknown command."
		}
	} else {
		if text == "" {
			return
		}
		reply, withKeyboard, done = userState.HandleText(text)
		if done {
			removeKeyboard = true
		}
	}

	if reply == "" {
		return
	}

	out := tgbotapi.NewMessage(chatID, reply)
	if removeKeyboard {
		out.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
	} else if withKeyboard {
		kb := mainKeyboard()
		out.ReplyMarkup = kb
	}

	if _, err := b.api.Send(out); err != nil {
		log.Printf("send error: %v", err)
	}

	b.save()
}

func main() {
	token := os.Getenv("TELEGRAM_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_TOKEN env var is not set")
	}

	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "."
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatalf("cannot create data dir %s: %v", dataDir, err)
	}
	dataPath := filepath.Join(dataDir, "conversationbot.json")

	storage := NewStorage(dataPath)

	botAPI, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatalf("failed to create bot api: %v", err)
	}
	botAPI.Debug = false

	log.Printf("Authorized on account %s", botAPI.Self.UserName)

	bot, err := NewBot(botAPI, storage)
	if err != nil {
		log.Fatalf("failed to create bot: %v", err)
	}

	if err := bot.Run(); err != nil {
		log.Fatalf("bot stopped with error: %v", err)
	}
}