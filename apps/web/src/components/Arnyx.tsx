import {
  createContext,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  ArrowUp,
  CornerDownLeft,
  SlidersHorizontal,
  ShieldCheck,
} from "./icons";
import { Link, useLocation } from "react-router-dom";
import { request, useAPI } from "../api";
import type { Reply } from "../types";
import { Button } from "./ui/button";

type Message = {
  role: "user" | "assistant";
  text: string;
  proposal?: boolean;
  confirmation?: Reply["confirmation"];
};
function shortTurn(text: string) {
  let value = "";
  for (const char of text) {
    if (new TextEncoder().encode(value + char).length > 800) break;
    value += char;
  }
  return value;
}
function useConversation() {
  const [messages, setMessages] = useState<Message[]>([]);
  const [confirming, setConfirming] = useState<number | null>(null);
  const [lastReply, setLastReply] = useState<Reply>();
  const client = useQueryClient();
  const location = useLocation();
  const runtime = useAPI<{
    mode: string;
    codex_available: boolean;
    arnyx_version?: number;
  }>("/runtime");
  const mutation = useMutation({
    retry: false,
    mutationFn: ({
      message,
      history,
    }: {
      message: string;
      history: Message[];
    }) =>
      request<Reply>("/arnyx/chat", {
        message,
        history: history
          .slice(-6)
          .map((m) => ({ role: m.role, content: shortTurn(m.text) })),
        view: {
          route: location.pathname,
          contribution_id:
            location.pathname.match(/^\/contributions\/([^/]+)$/)?.[1] || "",
        },
      }),
    onSuccess: async (reply) => {
      setLastReply(reply);
      setMessages(
        (m) =>
          [
            ...m,
            {
              role: "assistant",
              text: reply.message,
              proposal: !!reply.proposal,
              confirmation: reply.confirmation,
            },
          ].slice(-24) as Message[],
      );
      await client.invalidateQueries({ queryKey: ["/config/proposals"] });
    },
  });
  function send(text: string) {
    if (
      !text.trim() ||
      mutation.isPending ||
      confirming !== null ||
      !runtime.data?.arnyx_version
    )
      return;
    const message = text.trim();
    mutation.mutate({ message, history: messages });
    setMessages(
      (m) => [...m, { role: "user", text: message }].slice(-24) as Message[],
    );
  }
  async function confirm(index: number) {
    const c = messages[index]?.confirmation;
    if (!c || confirming !== null || mutation.isPending) return;
    if (
      c.action === "abandon" &&
      !window.confirm(
        "Abandon this contribution? Workspace and history will be preserved.",
      )
    )
      return;
    setConfirming(index);
    try {
      await request(
        "/contributions/" +
          encodeURIComponent(c.contribution_id) +
          "/" +
          c.action,
        { approved: true, constraints: c.constraints || "" },
      );
      setMessages((items) =>
        items.map((m, i) =>
          i === index
            ? {
                ...m,
                text: m.text + " Action accepted.",
                confirmation: undefined,
              }
            : m,
        ),
      );
      await client.invalidateQueries();
    } catch (e) {
      setMessages((items) => [
        ...items,
        { role: "assistant", text: (e as Error).message },
      ]);
    } finally {
      setConfirming(null);
    }
  }
  function clear() {
    if (!mutation.isPending && confirming === null) {
      setMessages([]);
      setLastReply(undefined);
      mutation.reset();
    }
  }
  return {
    messages,
    send,
    confirm,
    confirming,
    clear,
    mutation,
    runtime,
    lastReply,
    route: location.pathname,
  };
}
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
  const needsRestart = !!runtime.data && !runtime.data.arnyx_version;
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
            mutation.isPending || confirming !== null || !messages.length
          }
        >
          Clear chat
        </button>
      </div>
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
            if (input.trim() && !mutation.isPending) {
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
              confirming !== null ||
              needsRestart
            }
          >
            <ArrowUp size={18} />
          </Button>
        </form>
        <p>
          <ShieldCheck size={12} /> Changes wait for your approval{" "}
          <span title="Only compact current state and six short conversation turns are sent. No activity logs.">
            {lastReply?.context_bytes
              ? `${(lastReply.context_bytes / 1024).toFixed(1)} KB context`
              : "No activity logs"}
          </span>
        </p>
      </div>
    </div>
  );
}
