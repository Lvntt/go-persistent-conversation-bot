package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStartNewUser(t *testing.T) {
	u := NewUserState()
	u.Data = map[string]string{} // новый пользователь

	reply := u.HandleCommandStart()

	if u.State != StateChoosing {
		t.Fatalf("expected state %q, got %q", StateChoosing, u.State)
	}
	if !strings.Contains(reply, "I will hold a more complex conversation with you") {
		t.Fatalf("unexpected reply: %s", reply)
	}
}

func TestStartKnownUser(t *testing.T) {
	u := NewUserState()
	u.Data["age"] = "30"

	reply := u.HandleCommandStart()

	if !strings.Contains(reply, "You already told me your") {
		t.Fatalf("expected mention of existing data, got: %s", reply)
	}
	if !strings.Contains(reply, "age") {
		t.Fatalf("expected key 'age' in reply, got: %s", reply)
	}
}

func TestPredefinedFlow_Age(t *testing.T) {
	u := NewUserState()
	u.Data = map[string]string{}

	// /start
	_ = u.HandleCommandStart()
	if u.State != StateChoosing {
		t.Fatalf("expected state choosing after start, got %q", u.State)
	}

	// выбираем Age
	reply, withKeyboard, done := u.HandleText("Age")
	if done {
		t.Fatalf("conversation should not be done after choosing Age")
	}
	if u.State != StateTypingReply {
		t.Fatalf("expected state typing_reply, got %q", u.State)
	}
	if u.Choice != "age" {
		t.Fatalf("expected choice 'age', got %q", u.Choice)
	}
	if !strings.Contains(reply, "Your age?") {
		t.Fatalf("unexpected reply: %s", reply)
	}
	if withKeyboard {
		t.Fatalf("keyboard should not be shown at this step")
	}

	// вводим значение
	reply, withKeyboard, done = u.HandleText("30")
	if done {
		t.Fatalf("conversation should not be done yet")
	}
	if !withKeyboard {
		t.Fatalf("keyboard should be shown after saving info")
	}
	if u.State != StateChoosing {
		t.Fatalf("expected state choosing, got %q", u.State)
	}
	if v := u.Data["age"]; v != "30" {
		t.Fatalf("expected saved age '30', got %q", v)
	}
	if !strings.Contains(reply, "Neat! Just so you know") {
		t.Fatalf("unexpected reply: %s", reply)
	}

	// Done
	reply, withKeyboard, done = u.HandleText("Done")
	if !done {
		t.Fatalf("conversation should be done after Done")
	}
	if withKeyboard {
		t.Fatalf("keyboard should be removed after Done")
	}
	if !strings.Contains(reply, "I learned these facts about you") {
		t.Fatalf("unexpected reply on Done: %s", reply)
	}
}

func TestCustomCategoryFlow(t *testing.T) {
	u := NewUserState()
	u.Data = map[string]string{}

	_ = u.HandleCommandStart()

	// Something else...
	reply, withKeyboard, done := u.HandleText("Something else...")
	if done {
		t.Fatalf("should not be done")
	}
	if withKeyboard {
		t.Fatalf("keyboard should not be shown at this step")
	}
	if u.State != StateTypingChoice {
		t.Fatalf("expected typing_choice, got %q", u.State)
	}
	if !strings.Contains(reply, "please send me the category first") {
		t.Fatalf("unexpected reply: %s", reply)
	}

	// отправляем название категории
	reply, withKeyboard, done = u.HandleText("Most impressive skill")
	if done {
		t.Fatalf("should not be done")
	}
	if u.State != StateTypingReply {
		t.Fatalf("expected typing_reply, got %q", u.State)
	}
	if u.Choice != "most impressive skill" {
		t.Fatalf("expected choice to be lowercased, got %q", u.Choice)
	}
	if !strings.Contains(reply, "Your most impressive skill?") {
		t.Fatalf("unexpected reply: %s", reply)
	}

	// отправляем значение
	reply, withKeyboard, done = u.HandleText("Go programming")
	if done {
		t.Fatalf("should not be done yet")
	}
	if !withKeyboard {
		t.Fatalf("keyboard should be shown after saving custom info")
	}
	if u.State != StateChoosing {
		t.Fatalf("expected choosing, got %q", u.State)
	}
	if v := u.Data["most impressive skill"]; v != "go programming" {
		t.Fatalf("expected saved value 'go programming', got %q", v)
	}
	if !strings.Contains(reply, "this is what you already told me") {
		t.Fatalf("unexpected reply: %s", reply)
	}
}

func TestStorageSaveLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	st := NewStorage(path)

	users := map[int64]*UserState{
		42: {
			State:  StateChoosing,
			Choice: "",
			Data: map[string]string{
				"age": "30",
			},
		},
	}

	if err := st.Save(users); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	loaded, err := st.Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if len(loaded) != 1 {
		t.Fatalf("expected 1 user, got %d", len(loaded))
	}
	u, ok := loaded[42]
	if !ok {
		t.Fatalf("user 42 not found after load")
	}
	if v := u.Data["age"]; v != "30" {
		t.Fatalf("expected age 30 after load, got %q", v)
	}
}

func TestStorageLoadNonExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "no_such_file.json")

	st := NewStorage(path)
	users, err := st.Load()
	if err != nil {
		t.Fatalf("Load should succeed for non-existing file, got error: %v", err)
	}
	if len(users) != 0 {
		t.Fatalf("expected empty users for non-existing file, got %d", len(users))
	}
}

func TestFactsToStrEmpty(t *testing.T) {
	s := factsToStr(map[string]string{})
	if s != "\n\n" {
		t.Fatalf("expected two newlines, got %q", s)
	}
}

func TestMainNoToken(t *testing.T) {
	// Просто проверяем, что отсутствие TELEGRAM_TOKEN не приводит к панике при чтении env
	_ = os.Getenv("TELEGRAM_TOKEN")
}