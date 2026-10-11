import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useLocation } from "react-router-dom";
import { request, useAPI } from "../api";
import type { Reply } from "../types";

export type ChatMessage = { role: "user" | "assistant"; text: string; sequence?: number; afterSequence?: number; proposal?: boolean; confirmation?: Reply["confirmation"] };
export type WorkspaceTask = { id: string; conversation_id: string; directory: string; mode: string; prompt: string; network: boolean; status: string; activity: string; error?: string; updated_at: string };
type SavedChat = { id: string; title: string; updated_at: string };
type Detail = { messages: ChatMessage[]; tasks: WorkspaceTask[] };
function storedChat() { try { return localStorage.getItem("forgeflow.arnyx.conversation") || ""; } catch { return ""; } }
function shortTurn(text: string) {
  let value = "";
  for (const char of text) { if (new TextEncoder().encode(value + char).length > 800) break; value += char; }
  return value;
}
export function useArnyxConversation() {
  const [id, setID] = useState(storedChat);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [confirming, setConfirming] = useState<number | null>(null);
  const [lastReply, setLastReply] = useState<Reply>();
  const [modeChoice, setMode] = useState("auto");
  const [directory, setDirectory] = useState("");
  const [network, setNetwork] = useState(false);
  const [historyOpen, setHistoryOpen] = useState(false);
  const [historyOffset, setHistoryOffset] = useState(0);
  const [olderAvailable, setOlderAvailable] = useState(false);
  const [historyError, setHistoryError] = useState("");
  const client = useQueryClient();
  const location = useLocation();
  const runtime = useAPI<{ mode: string; codex_available: boolean; arnyx_version?: number; arnyx_workspace_available?: boolean; project_root?: string }>("/runtime");
  const workspaceAvailable = !!runtime.data?.arnyx_workspace_available;
  const mode = modeChoice === "auto" ? (workspaceAvailable ? "inspect" : "controls") : modeChoice;
  const history = useQuery({ queryKey: ["/arnyx/conversations", historyOffset], queryFn: () => request<SavedChat[]>(`/arnyx/conversations?offset=${historyOffset}`), enabled: (runtime.data?.arnyx_version || 0) >= 2 });
  const detail = useQuery({
    queryKey: [`/arnyx/conversations/${id}`],
    queryFn: () => request<Detail>(`/arnyx/conversations/${encodeURIComponent(id)}`),
    enabled: !!id && (runtime.data?.arnyx_version || 0) >= 2,
    refetchInterval: q => q.state.data?.tasks.some(t => t.status === "RUNNING") ? 1000 : false,
  });
  useEffect(() => {
    if (!detail.data || !detail.data.messages.length) return;
    const fetched = detail.data.messages;
    setMessages(previous => {
      const first = fetched[0]?.sequence || 0;
      const older = previous.filter(m => m.sequence && m.sequence < first);
      // A GET can finish between saving the user turn and saving its reply.
      // Keep optimistic turns until a newer persisted row acknowledges them.
      const pending = previous.filter(p => !p.sequence && !fetched.some(m => (m.sequence || 0) > (p.afterSequence || 0) && m.role === p.role && m.text === p.text));
      return [...older, ...fetched.map((m,index) => {
        const existing = previous.find(p => p.sequence === m.sequence || (p.role === "assistant" && p.text === m.text));
        const currentReply = index === fetched.length - 1 && m.role === "assistant" && m.text === lastReply?.message ? lastReply : undefined;
        return { ...m, proposal: currentReply ? !!currentReply.proposal : existing?.proposal, confirmation: currentReply ? currentReply.confirmation : existing?.confirmation };
      }),...pending];
    });
    setOlderAvailable(fetched.length === 100);
  }, [detail.data,lastReply]);
  const tasks = detail.data?.tasks || [];
  const running = tasks.some(t => t.status === "RUNNING");
  function selectChat(next: string) {
    setID(next);setMessages([]);setLastReply(undefined);setOlderAvailable(false);setHistoryError("");setHistoryOpen(false);
    try { if (next) localStorage.setItem("forgeflow.arnyx.conversation", next); else localStorage.removeItem("forgeflow.arnyx.conversation"); } catch { /* History also lives on the server. */ }
  }
  const view = { route: location.pathname, contribution_id: location.pathname.match(/^\/contributions\/([^/]+)$/)?.[1] || "" };
  const focused = useQuery({ queryKey: [`/contributions/${view.contribution_id}`], queryFn: () => request<{workspace?: string}>(`/contributions/${encodeURIComponent(view.contribution_id)}`), enabled: !!view.contribution_id });
  const mutation = useMutation({
    retry: false,
    mutationFn: async ({ message, previous }: { message: string; previous: ChatMessage[] }) => {
      let conversationID = id;
      if ((runtime.data?.arnyx_version || 0) >= 2 && !conversationID) {
        const saved = await request<SavedChat>("/arnyx/conversations", {});
        conversationID = saved.id;setID(saved.id);
        try { localStorage.setItem("forgeflow.arnyx.conversation", saved.id); } catch { /* optional */ }
      }
      if (mode !== "controls") {
        const task = await request<WorkspaceTask>(`/arnyx/conversations/${encodeURIComponent(conversationID)}/tasks`, { message, directory, mode, network, view });
        return { task, conversationID };
      }
      const reply = await request<Reply>("/arnyx/chat", { message, history: previous.slice(-6).map(m => ({role: m.role, content: shortTurn(m.text)})), view, ...(conversationID ? { conversation_id: conversationID } : {}) });
      return { reply, conversationID };
    },
    onSuccess: async result => {
      if (result.reply) {
        const reply = result.reply;setLastReply(reply);
        setMessages(m => {
          const response: ChatMessage = {role:"assistant",text:reply.message,proposal:!!reply.proposal,confirmation:reply.confirmation,afterSequence:Math.max(0,...m.map(item => item.sequence || 0))};
          return m.at(-1)?.role === "assistant" && m.at(-1)?.text === reply.message ? [...m.slice(0,-1),{...m.at(-1),...response}] : [...m,response];
        });
        await client.invalidateQueries({ queryKey:["/config/proposals"] });
      } else {
        await client.invalidateQueries({queryKey:[`/arnyx/conversations/${result.conversationID}`]});
      }
      await client.invalidateQueries({queryKey:["/arnyx/conversations"]});
    },
  });
  const taskAction = useMutation({
    mutationFn: ({task,action}:{task:WorkspaceTask;action:"approve"|"stop"}) => request<WorkspaceTask>(`/arnyx/tasks/${task.id}/${action}`,{approved:action==="approve"}),
    onSuccess: async () => { await client.invalidateQueries({queryKey:[`/arnyx/conversations/${id}`]});await client.invalidateQueries({queryKey:["/arnyx/conversations"]}); },
  });
  function send(text: string) {
    if (!text.trim() || mutation.isPending || running || confirming !== null || detail.isFetching || !runtime.data?.arnyx_version) return;
    const message = text.trim();mutation.mutate({message,previous:messages});setMessages(m => [...m,{role:"user",text:message,afterSequence:Math.max(0,...m.map(item => item.sequence || 0))}]);
  }
  async function confirm(index: number) {
    const c = messages[index]?.confirmation;if (!c || confirming !== null || mutation.isPending) return;
    if (c.action === "abandon" && !window.confirm("Abandon this contribution? Workspace and history will be preserved.")) return;
    setConfirming(index);
    try {
      await request(`/contributions/${encodeURIComponent(c.contribution_id)}/${c.action}`,{approved:true,constraints:c.constraints||""});
      setLastReply(reply => reply ? {...reply,confirmation:undefined} : reply);
      setMessages(items => items.map((m,i) => i===index ? {...m,text:m.text+" Action accepted.",confirmation:undefined}:m));
      await client.invalidateQueries({predicate:q => !String(q.queryKey[0]).startsWith("/arnyx/conversations/")});
    } catch(e) {setMessages(items => [...items,{role:"assistant",text:(e as Error).message}]);}finally{setConfirming(null);}
  }
  async function loadOlder() {
    const first = messages.find(m => m.sequence)?.sequence;if (!first || !id) return;
    try {const page = await request<Detail>(`/arnyx/conversations/${id}?before=${first}`);setMessages(m => [...page.messages,...m]);setOlderAvailable(page.messages.length===100);}catch(e){setHistoryError((e as Error).message);}
  }
  function clear() { if (!mutation.isPending && confirming===null) {selectChat("");setHistoryOffset(0);mutation.reset();taskAction.reset();} }
  return { id,messages,send,confirm,confirming,clear,mutation,runtime,lastReply,route:location.pathname,mode,setMode,directory,setDirectory,network,setNetwork,workspaceAvailable,tasks,running,taskAction,history,historyOpen,setHistoryOpen,historyOffset,setHistoryOffset,selectChat,detail,loadOlder,olderAvailable,historyError,focusedWorkspace:focused.data?.workspace };
}
