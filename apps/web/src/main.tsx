import { For, Show, createSignal, onMount } from "solid-js";
import { render } from "solid-js/web";
import "./styles.css";

const API = import.meta.env.VITE_API_URL || "http://localhost:8080";

type Conversation = { id: string; title: string; provider: string; model: string; updated_at: string };
type Message = { id?: number; role: "user" | "assistant" | "system"; content: string; pending?: boolean; failed?: boolean };
type Metrics = { avg_latency_ms: number; throughput_1h: number; error_rate: number; tokens: number; providers: Record<string, number> };

function App() {
  const [conversations, setConversations] = createSignal<Conversation[]>([]);
  const [active, setActive] = createSignal<string>("");
  const [messages, setMessages] = createSignal<Message[]>([]);
  const [input, setInput] = createSignal("");
  const [provider, setProvider] = createSignal("openai");
  const [streaming, setStreaming] = createSignal(false);
  const [lastError, setLastError] = createSignal("");
  const [status, setStatus] = createSignal("Idle");
  const [metrics, setMetrics] = createSignal<Metrics>({ avg_latency_ms: 0, throughput_1h: 0, error_rate: 0, tokens: 0, providers: {} });
  let aborter: AbortController | undefined;
  let messageLoadSeq = 0;

  onMount(async () => {
    const list = await refreshConversations();
    if (list[0]) await selectConversation(list[0].id);
    await refreshMetrics();
    setInterval(refreshMetrics, 5000);
  });

  async function refreshConversations() {
    const res = await fetch(`${API}/api/conversations`);
    const list = await res.json();
    setConversations(list);
    return list as Conversation[];
  }

  async function selectConversation(id: string) {
    const seq = ++messageLoadSeq;
    setActive(id);
    setLastError("");
    setStatus("Loading conversation");
    setMessages([]);
    const res = await fetch(`${API}/api/conversations/${id}/messages`);
    if (!res.ok) {
      if (seq === messageLoadSeq) {
        setLastError(await readHTTPError(res));
        setStatus("Failed");
      }
      return;
    }
    const items = await res.json();
    if (seq === messageLoadSeq && active() === id) {
      setMessages(items);
      setStatus("Idle");
    }
  }

  async function refreshMetrics() {
    const res = await fetch(`${API}/api/metrics`);
    if (res.ok) setMetrics(await res.json());
  }

  async function newConversation() {
    const id = await createConversation();
    messageLoadSeq++;
    setActive(id);
    setMessages([]);
    setLastError("");
    setStatus("Idle");
    await refreshConversations();
  }

  async function createConversation() {
    const res = await fetch(`${API}/api/conversations`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ title: "Inference trace", provider: provider(), model: "gpt-4.1-mini" }),
    });
    if (!res.ok) throw new Error(await readHTTPError(res));
    const body = await res.json();
    return body.id as string;
  }

  async function send(text = input()) {
    const content = text.trim();
    if (!content || streaming()) return;
    let id = active();
    setInput("");
    setLastError("");
    setStreaming(true);
    setStatus("Preparing request");
    aborter = new AbortController();
    setMessages((m) => [...m, { role: "user", content }, { role: "assistant", content: "", pending: true }]);
    try {
      if (!id) {
        id = await createConversation();
        messageLoadSeq++;
        setActive(id);
        await refreshConversations();
      }
      setStatus("Opening SSE stream");
      const res = await fetch(`${API}/api/conversations/${id}/messages`, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ content, provider: provider(), model: "gpt-4.1-mini" }),
        signal: aborter.signal,
      });
      if (!res.ok) throw new Error(await readHTTPError(res));
      if (!res.body) throw new Error("stream unavailable");
      setStatus("Streaming response");
      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let buffer = "";
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });
        const events = buffer.split("\n\n");
        buffer = events.pop() || "";
        for (const evt of events) {
          const line = evt.split("\n").find((l) => l.startsWith("data: "));
          if (!line) continue;
          const chunk = JSON.parse(line.slice(6));
          if (chunk.error) throw new Error(chunk.error);
          if (chunk.delta) setMessages((m) => updateLast(m, chunk.delta));
        }
      }
      setMessages((m) => m.map((msg, i) => (i === m.length - 1 ? { ...msg, pending: false } : msg)));
      setStatus("Completed");
      await refreshConversations();
      await refreshMetrics();
    } catch (err) {
      const text = err instanceof Error ? err.message : "Interrupted";
      setLastError(text);
      setStatus("Failed");
      setMessages((m) => m.map((msg, i) => (i === m.length - 1 ? { ...msg, pending: false, failed: true, content: msg.content || text } : msg)));
    } finally {
      setStreaming(false);
      aborter = undefined;
    }
  }

  function cancel() {
    aborter?.abort();
    setStatus("Cancelling");
    setStreaming(false);
  }

  function retry() {
    const lastUser = [...messages()].reverse().find((m) => m.role === "user");
    if (lastUser) send(lastUser.content);
  }

  return (
    <div class="min-h-screen bg-zinc-950 text-zinc-100">
      <div class="grid min-h-screen grid-cols-1 md:grid-cols-[280px_1fr]">
        <aside class="border-r border-zinc-800 bg-zinc-900/80 p-4">
          <div class="mb-4 flex items-center justify-between">
            <h1 class="text-lg font-semibold">Ollive</h1>
            <button class="btn" onClick={newConversation}>New</button>
          </div>
          <select class="field mb-4" value={provider()} onInput={(e) => setProvider(e.currentTarget.value)}>
            <option value="openai">OpenAI GPT-4.1-mini</option>
            <option value="claude">Claude skeleton</option>
            <option value="gemini">Gemini skeleton</option>
          </select>
          <div class="space-y-2">
            <For each={conversations()}>{(c) =>
              <button class={`w-full rounded border p-3 text-left text-sm ${active() === c.id ? "border-emerald-500 bg-emerald-950/40" : "border-zinc-800 bg-zinc-950"}`} onClick={() => selectConversation(c.id)}>
                <div class="truncate font-medium">{c.title}</div>
                <div class="text-xs text-zinc-400">{c.provider} · {c.model}</div>
              </button>
            }</For>
          </div>
        </aside>
        <main class="grid grid-rows-[auto_1fr_auto]">
          <Dashboard metrics={metrics()} />
          <section class="overflow-y-auto p-4">
            <div class="mx-auto max-w-4xl space-y-3">
              <For each={messages()}>{(m) =>
                <div class={`rounded border p-3 ${m.role === "user" ? "ml-auto max-w-2xl border-sky-700 bg-sky-950/40" : "mr-auto max-w-3xl border-zinc-800 bg-zinc-900"}`}>
                  <div class="mb-1 text-xs uppercase text-zinc-500">{m.role}{m.pending ? " streaming" : ""}{m.failed ? " failed" : ""}</div>
                  <div class="whitespace-pre-wrap text-sm leading-6">{m.content}</div>
                </div>
              }</For>
            </div>
          </section>
          <form class="border-t border-zinc-800 p-4" onSubmit={(e) => { e.preventDefault(); send(); }}>
            <Show when={lastError()}>
              <div class="mx-auto mb-3 max-w-4xl rounded border border-rose-800 bg-rose-950/40 px-3 py-2 text-sm text-rose-100">{lastError()}</div>
            </Show>
            <div class="mx-auto mb-2 max-w-4xl text-xs text-zinc-500">{status()}</div>
            <div class="mx-auto flex max-w-4xl gap-2">
              <input class="field" value={input()} onInput={(e) => setInput(e.currentTarget.value)} placeholder="Send an inference request..." />
              <Show when={streaming()} fallback={<button class="btn-primary" type="button" onClick={() => send()}>Stream</button>}>
                <button class="btn-danger" type="button" onClick={cancel}>Cancel</button>
              </Show>
              <button class="btn" type="button" onClick={retry}>Retry</button>
            </div>
          </form>
        </main>
      </div>
    </div>
  );
}

function Dashboard(props: { metrics: Metrics }) {
  const providerRows = () => Object.entries(props.metrics.providers || {});
  const maxProvider = () => Math.max(1, ...providerRows().map(([, v]) => v));
  return (
    <section class="border-b border-zinc-800 p-4">
      <div class="mx-auto grid max-w-5xl grid-cols-2 gap-3 md:grid-cols-5">
        <Metric label="Latency" value={`${props.metrics.avg_latency_ms}ms`} />
        <Metric label="Throughput 1h" value={props.metrics.throughput_1h} />
        <Metric label="Error rate" value={`${props.metrics.error_rate.toFixed(1)}%`} />
        <Metric label="Tokens" value={props.metrics.tokens} />
        <div class="rounded border border-zinc-800 bg-zinc-900 p-3">
          <div class="text-xs text-zinc-500">Providers</div>
          <div class="mt-2 space-y-2 text-xs">
            <Show when={providerRows().length} fallback={<div class="text-zinc-600">No traffic yet</div>}>
              <For each={providerRows()}>{([k, v]) =>
                <div>
                  <div class="mb-1 flex justify-between"><span>{k}</span><span>{v}</span></div>
                  <div class="h-1.5 rounded bg-zinc-800"><div class="h-1.5 rounded bg-emerald-500" style={{ width: `${Math.max(8, (v / maxProvider()) * 100)}%` }} /></div>
                </div>
              }</For>
            </Show>
          </div>
        </div>
      </div>
      <div class="mx-auto mt-3 grid max-w-5xl grid-cols-1 gap-3 md:grid-cols-2">
        <Bar label="Latency budget" value={Math.min(100, props.metrics.avg_latency_ms / 20)} caption={`${props.metrics.avg_latency_ms}ms avg`} />
        <Bar label="Error pressure" value={Math.min(100, props.metrics.error_rate)} caption={`${props.metrics.error_rate.toFixed(1)}% failed`} danger={props.metrics.error_rate > 5} />
      </div>
    </section>
  );
}

function Metric(props: { label: string; value: string | number }) {
  return <div class="rounded border border-zinc-800 bg-zinc-900 p-3"><div class="text-xs text-zinc-500">{props.label}</div><div class="mt-1 text-xl font-semibold">{props.value}</div></div>;
}

function updateLast(messages: Message[], delta: string) {
  return messages.map((m, i) => i === messages.length - 1 ? { ...m, content: m.content + delta } : m);
}

function Bar(props: { label: string; value: number; caption: string; danger?: boolean }) {
  return (
    <div class="rounded border border-zinc-800 bg-zinc-900 p-3">
      <div class="mb-2 flex justify-between text-xs"><span class="text-zinc-500">{props.label}</span><span>{props.caption}</span></div>
      <div class="h-2 rounded bg-zinc-800">
        <div class={`h-2 rounded ${props.danger ? "bg-rose-500" : "bg-sky-500"}`} style={{ width: `${Math.max(3, props.value)}%` }} />
      </div>
    </div>
  );
}

async function readHTTPError(res: Response) {
  const body = await res.text();
  try {
    const parsed = JSON.parse(body);
    return parsed.error || parsed.message || body || `request failed with ${res.status}`;
  } catch {
    return body || `request failed with ${res.status}`;
  }
}

render(() => <App />, document.getElementById("root")!);
