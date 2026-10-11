import {
  createContext,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { useArnyxConversation as useConversation } from "./useArnyxConversation";
import {
  ArrowUp,
  CornerDownLeft,
  SlidersHorizontal,
  ShieldCheck,
} from "./icons";
import { Link } from "react-router-dom";
import { Button } from "./ui/button";

const Conversation = createContext<ReturnType<typeof useConversation> | null>(
  null,
);
export function ArnyxProvider({ children }: { children: ReactNode }) {
  const chat = useConversation();
  return <Conversation.Provider value={chat}>{children}</Conversation.Provider>;
}
export default function Arnyx() {
  const chat = useContext(Conversation);
  if (!chat) throw new Error("ArnyxProvider is missing");
  const {
    messages,
    mutation,
    runtime,
    send,
    confirm,
    confirming,
    clear,
    lastReply,
  } = chat;
  const [input, setInput] = useState("");
  const log = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const el = log.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [messages, mutation.isPending]);
  const needsRestart = !!runtime.data && (runtime.data.arnyx_version || 0) < 2;
  const live = runtime.data?.mode === "live" && runtime.data.codex_available;
  return (
    <div className="arnyx-chat">
      <div className="arnyx-context">
        <span className="arnyx-status-dot" />
        <span>
          {needsRestart
            ? "Backend restart needed"
            : lastReply?.engine === "codex"
              ? "LLM responded"
              : live
                ? "Local LLM available"
                : runtime.isPending
                  ? "Checking connection…"
                  : "Offline assistant"}
        </span>
        <button
          onClick={clear}
          disabled={
            mutation.isPending || confirming !== null
          }
        >
          New chat
        </button>
        <button onClick={() => chat.setHistoryOpen(!chat.historyOpen)} aria-expanded={chat.historyOpen} disabled={mutation.isPending}>Previous chats</button>
      </div>
      {chat.historyOpen && <div className="arnyx-history" aria-label="Previous conversations">
        {chat.history.isPending && <p role="status">Loading conversations…</p>}
        {chat.history.error && <p role="alert">{chat.history.error.message}</p>}
        {chat.history.data?.length === 0 && <p>No saved conversations yet.</p>}
        {chat.history.data?.map(c => <button key={c.id} className={c.id === chat.id ? "is-active" : ""} onClick={() => chat.selectChat(c.id)}>
          <span>{c.title}</span><time>{new Date(c.updated_at).toLocaleString()}</time>
        </button>)}
        <div className="arnyx-history-pages">
          {chat.historyOffset > 0 && <button onClick={() => chat.setHistoryOffset(Math.max(0, chat.historyOffset - 50))}>Newer chats</button>}
          {chat.history.data?.length === 50 && <button onClick={() => chat.setHistoryOffset(chat.historyOffset + 50)}>Older chats</button>}
        </div>
      </div>}
      {chat.workspaceAvailable && <div className="arnyx-access">
        <label>Access mode<select aria-label="Arnyx access mode" value={chat.mode} onChange={e => chat.setMode(e.target.value)} disabled={mutation.isPending || chat.running}>
          <option value="inspect">Inspect files</option><option value="edit">Make changes</option><option value="controls">ForgeFlow controls</option>
        </select></label>
        {chat.mode !== "controls" && <>
          <label>Project or laptop folder<input aria-label="Arnyx workspace folder" value={chat.directory} placeholder={runtime.data?.project_root || "Absolute folder path"} onChange={e => chat.setDirectory(e.target.value)} disabled={mutation.isPending || chat.running} /></label>
          <div className="arnyx-folder-shortcuts"><button onClick={() => chat.setDirectory(runtime.data?.project_root || "")} disabled={chat.running}>ForgeFlow project</button>
            {chat.focusedWorkspace && <button onClick={() => chat.setDirectory(chat.focusedWorkspace!)} disabled={chat.running}>Current contribution</button>}
          </div>
          <label className="arnyx-network"><input type="checkbox" checked={chat.network} onChange={e => chat.setNetwork(e.target.checked)} disabled={mutation.isPending || chat.running} />Allow network and dependency downloads</label>
          <p>{chat.mode === "edit" ? "Your request and folder will appear for approval before Codex edits files." : "Codex can read local files and use diagnostic tools. Project files remain read-only."}</p>
        </>}
      </div>}
      {needsRestart && (
        <p className="arnyx-restart" role="status">
          Restart ForgeFlow once to load Arnyx’s updated backend.
        </p>
      )}
      <div
        className="arnyx-messages"
        ref={log}
        role="log"
        aria-live="polite"
        aria-label="Conversation with Arnyx"
      >
        {chat.detail.isPending && chat.id && <p role="status">Loading saved conversation…</p>}
        {chat.detail.error && <div role="alert" className="arnyx-error"><p>{chat.detail.error.message}</p><button onClick={clear}>Start a new chat</button></div>}
        {chat.olderAvailable && <button className="arnyx-load-older" onClick={() => void chat.loadOlder()}>Load earlier messages</button>}
        {chat.historyError && <p role="alert">{chat.historyError}</p>}
        {!messages.length && (
          <div className="arnyx-welcome">
            <div className="arnyx-welcome-avatar">
              <img src="/images/arnyx-avatar.svg" alt="" />
              <span>✦</span>
            </div>
            <span className="eyebrow">YOUR WORKSPACE COMPANION</span>
            <h3>
              A little help,
              <br />
              right here.
            </h3>
            <p>
              Tell me what’s on your mind. I can check your current work,
              explain a blocker, or help tune your configuration.
            </p>
            <div className="arnyx-suggestions">
              {[
                "What needs my attention?",
                "What are my current languages?",
                "Add Rust",
              ].map((x) => (
                <button
                  key={x}
                  disabled={
                    mutation.isPending || confirming !== null || needsRestart
                  }
                  onClick={() => send(x)}
                >
                  {x}
                  <CornerDownLeft size={13} />
                </button>
              ))}
            </div>
          </div>
        )}
        {messages.map((m, i) => (
          <div className={"arnyx-message arnyx-message-" + m.role} key={i}>
            <span className="eyebrow">
              {m.role === "user" ? "You" : "Arnyx"}
            </span>
            <p>{m.text}</p>
            {m.proposal && (
              <Link className="arnyx-proposal" to="/configuration">
                <SlidersHorizontal size={16} />
                Review configuration proposal →
              </Link>
            )}
            {m.confirmation && (
              <div className="arnyx-confirmation">
                <span className="eyebrow">APPROVAL REQUIRED</span>
                <p>
                  <code>{m.confirmation.contribution_id}</code>
                </p>
                {m.confirmation.constraints && (
                  <p>{m.confirmation.constraints}</p>
                )}
                <p>
                  {m.confirmation.action === "prepare-pr"
                    ? "Prepare a draft for review. PR submission requires its separate approval."
                    : `Request ${m.confirmation.action.replaceAll("-", " ")} for this contribution.`}
                </p>
                <Button
                  disabled={confirming !== null || mutation.isPending}
                  onClick={() => confirm(i)}
                >
                  {confirming === i ? "Requesting…" : m.confirmation.label}
                </Button>
              </div>
            )}
          </div>
        ))}
        {chat.tasks.filter(t => t.status === "RUNNING" || t.status === "AWAITING_APPROVAL").map(t => <div key={t.id} className="arnyx-confirmation arnyx-task">
          <span className="eyebrow">{t.status === "RUNNING" ? "CODEX RUNNING" : "APPROVAL REQUIRED"}</span>
          <p>{t.prompt}</p><code>{t.directory}</code>
          <p>{t.mode === "edit" ? "Edits in this folder" : "Read-only inspection"}{t.network ? " · network allowed" : " · tool network disabled"}</p>
          <p role="status">{t.activity}</p>
          {t.status === "RUNNING" ? <Button disabled={chat.taskAction.isPending} onClick={() => chat.taskAction.mutate({task:t,action:"stop"})}>Stop Codex task</Button> : <div className="arnyx-task-actions">
            <Button disabled={chat.taskAction.isPending || chat.running} onClick={() => chat.taskAction.mutate({task:t,action:"approve"})}>Approve changes in this folder</Button>
            <Button variant="secondary" disabled={chat.taskAction.isPending} onClick={() => chat.taskAction.mutate({task:t,action:"stop"})}>Cancel request</Button>
          </div>}
        </div>)}
        {chat.taskAction.error && <p className="arnyx-error" role="alert">{chat.taskAction.error.message}</p>}
        {mutation.isPending && (
          <div className="arnyx-thinking" role="status">
            <span />
            <span />
            <span />
            <p>Thinking with current workspace state…</p>
          </div>
        )}
        {mutation.error && (
          <div className="arnyx-error" role="alert">
            <p>{mutation.error.message}</p>
            <span>Your message is above. You can send it again.</span>
          </div>
        )}
      </div>
      <div className="arnyx-compose">
        <form
          onSubmit={(e) => {
            e.preventDefault();
            if (input.trim() && !mutation.isPending && !chat.running && !chat.detail.isFetching) {
              send(input);
              setInput("");
            }
          }}
        >
          <input
            data-autofocus
            aria-label="Message Arnyx"
            placeholder="Ask Arnyx anything…"
            value={input}
            maxLength={2000}
            onChange={(e) => setInput(e.target.value)}
          />
          <Button
            aria-label="Send message"
            size="small"
            disabled={
              !input.trim() ||
              mutation.isPending ||
              chat.running ||
              chat.detail.isFetching ||
              confirming !== null ||
              needsRestart
            }
          >
            <ArrowUp size={18} />
          </Button>
        </form>
        <p>
          <ShieldCheck size={12} /> Changes wait for your approval{" "}
          <span title="Saved history stays in ForgeFlow. Codex receives bounded recent context and can inspect selected project files.">
            {lastReply?.context_bytes
              ? `${(lastReply.context_bytes / 1024).toFixed(1)} KB context`
              : "Chats saved locally"}
          </span>
        </p>
      </div>
    </div>
  );
}
