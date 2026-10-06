import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { ArrowUp, Bot, CornerDownLeft } from "lucide-react";
import { Link } from "react-router-dom";
import { request, useAPI } from "../api";
import type { Reply } from "../types";
import { Badge } from "./primitives";
import { Button } from "./ui/button";
export default function Operator() {
  const [input, setInput] = useState("");
  const [messages, setMessages] = useState<
    { role: string; text: string; proposal: boolean; confirmation?: Reply["confirmation"] }[]
  >([]);
  const client = useQueryClient();
  const runtime = useAPI<{mode: string; codex_available: boolean}>("/runtime");
  const mutation = useMutation({
    mutationFn: (message: string) =>
      request<Reply>("/operator/chat", { message }),
    onSuccess: async (reply) => {
      setMessages((m) => [
        ...m,
        { role: "operator", text: reply.message, proposal: !!reply.proposal, confirmation: reply.confirmation },
      ]);
      await client.invalidateQueries({ queryKey: ["/config/proposals"] });
    },
  });
  function send(text: string) {
    if (!text.trim() || mutation.isPending) return;
    setMessages((m) => [...m, { role: "you", text, proposal: false }]);
    setInput("");
    mutation.mutate(text);
  }
  return (
    <div className="operator">
      <div className="operator-intro">
        <span className="operator-glyph">
          <Bot size={24} />
        </span>
        <h3>A little more control.</h3>
        <p>
          Inspect your system and propose changes through controlled application
          actions.
        </p>
        <Badge>{runtime.data?.mode === "live" && runtime.data?.codex_available ? "Codex Operator · controlled actions" : "Deterministic Operator"}</Badge>
      </div>
      <div className="operator-suggestions">
        {[
          "What are my agents doing?",
          "What are my current languages?",
          "Add Rust",
        ].map((x) => (
          <button key={x} disabled={mutation.isPending} onClick={() => send(x)}>
            {x}
            <CornerDownLeft size={12} />
          </button>
        ))}
      </div>
      <div className="messages" role="log" aria-live="polite">
        {messages.map((m, i) => (
          <div className={"message message-" + m.role} key={i}>
            <span className="eyebrow">{m.role}</span>
            <p>{m.text}</p>
            {m.proposal && (
              <Link className="text-link" to="/configuration">
                Review configuration proposal →
              </Link>
            )}
            {m.confirmation && <Button onClick={async () => {
              if (m.confirmation?.action === "abandon" && !window.confirm("Abandon this contribution? Workspace and history will be preserved.")) return;
              try { await request("/contributions/" + encodeURIComponent(m.confirmation!.contribution_id) + "/" + m.confirmation!.action, {approved: true, constraints: m.confirmation!.constraints || ""}); await client.invalidateQueries(); setMessages(items => items.map((item,index) => index === i ? {...item, text: item.text + " Action accepted.", confirmation: undefined} : item)); }
              catch(e) { setMessages(items => [...items, {role: "operator", text: (e as Error).message, proposal: false}]); }
            }}>{m.confirmation.label}</Button>}
          </div>
        ))}
        {mutation.isPending && <p className="muted">Inspecting…</p>}
        {mutation.error && (
          <p className="error" role="alert">
            {mutation.error.message}
          </p>
        )}
      </div>
      <form
        className="operator-input"
        onSubmit={(e) => {
          e.preventDefault();
          send(input);
        }}
      >
        <input
          aria-label="Message Operator"
          placeholder="Ask about your control plane…"
          value={input}
          maxLength={2000}
          onChange={(e) => setInput(e.target.value)}
        />
        <Button
          aria-label="Send message"
          size="small"
          disabled={!input.trim() || mutation.isPending}
        >
          <ArrowUp size={16} />
        </Button>
      </form>
      <p className="operator-footnote">
        Configuration changes always wait for Apply.
      </p>
    </div>
  );
}
