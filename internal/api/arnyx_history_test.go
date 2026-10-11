package api

import (
	"encoding/json"
	"forgeflow/internal/storage"
	"testing"
)

func TestArnyxHTTPHistoryReopensAndPreservesPriorConversations(t *testing.T) {
	server := testServer(t)
	response := post(t, server, "/api/arnyx/conversations", `{}`, server.URL)
	if response.StatusCode != 200 {
		t.Fatal(response.StatusCode)
	}
	var conversation storage.ArnyxConversation
	if e := json.NewDecoder(response.Body).Decode(&conversation); e != nil {
		t.Fatal(e)
	}
	response.Body.Close()
	body, _ := json.Marshal(map[string]any{"message": "What are my current languages?", "conversation_id": conversation.ID})
	response = post(t, server, "/api/arnyx/chat", string(body), server.URL)
	if response.StatusCode != 200 {
		t.Fatal(response.StatusCode)
	}
	response.Body.Close()
	response, e := server.Client().Get(server.URL + "/api/arnyx/conversations/" + conversation.ID)
	if e != nil {
		t.Fatal(e)
	}
	var details struct {
		Messages []storage.ArnyxMessage `json:"messages"`
	}
	if e = json.NewDecoder(response.Body).Decode(&details); e != nil {
		t.Fatal(e)
	}
	response.Body.Close()
	if len(details.Messages) != 2 || details.Messages[0].Role != "user" || details.Messages[1].Role != "assistant" {
		t.Fatal(details)
	}
	response = post(t, server, "/api/arnyx/conversations", `{}`, server.URL)
	response.Body.Close()
	response, e = server.Client().Get(server.URL + "/api/arnyx/conversations")
	if e != nil {
		t.Fatal(e)
	}
	var chats []storage.ArnyxConversation
	json.NewDecoder(response.Body).Decode(&chats)
	response.Body.Close()
	if len(chats) != 2 {
		t.Fatal("new conversation erased history", chats)
	}
	response = post(t, server, "/api/arnyx/conversations", `{}`, "https://unrelated.example")
	if response.StatusCode != 403 {
		t.Fatal("cross-origin mutation allowed", response.StatusCode)
	}
	response.Body.Close()
	response = post(t, server, "/api/arnyx/tasks/unknown/approve", `{"approved":true}`, server.URL)
	if response.StatusCode != 503 {
		t.Fatal("missing assistant accepted task", response.StatusCode)
	}
	response.Body.Close()
}
