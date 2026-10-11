CREATE TABLE arnyx_conversations (id TEXT PRIMARY KEY, title TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
CREATE TABLE arnyx_messages (sequence INTEGER PRIMARY KEY AUTOINCREMENT, conversation_id TEXT NOT NULL REFERENCES arnyx_conversations(id), role TEXT NOT NULL CHECK(role IN ('user','assistant')), text TEXT NOT NULL, created_at TEXT NOT NULL);
CREATE INDEX arnyx_messages_conversation ON arnyx_messages(conversation_id,sequence);
CREATE TABLE arnyx_tasks (id TEXT PRIMARY KEY, conversation_id TEXT NOT NULL REFERENCES arnyx_conversations(id), data TEXT NOT NULL);
CREATE INDEX arnyx_tasks_conversation ON arnyx_tasks(conversation_id);
