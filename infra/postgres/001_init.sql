create extension if not exists pgcrypto;

create table if not exists conversations (
	id uuid primary key default gen_random_uuid(),
	title text not null,
	provider text not null default 'openai',
	model text not null default 'gpt-4.1-mini',
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create table if not exists messages (
	id bigserial primary key,
	conversation_id uuid not null references conversations(id) on delete cascade,
	role text not null check (role in ('system','user','assistant')),
	content text not null,
	created_at timestamptz not null default now()
);

create table if not exists ingestion_events (
	id bigserial primary key,
	event_id text not null unique,
	subject text not null,
	payload jsonb not null,
	status text not null,
	created_at timestamptz not null default now()
);

create table if not exists inference_logs (
	id text primary key,
	conversation_id uuid references conversations(id) on delete set null,
	provider text not null,
	model text not null,
	status text not null,
	error text,
	latency_ms bigint not null,
	input_tokens int not null default 0,
	output_tokens int not null default 0,
	input_preview text,
	output_preview text,
	metadata jsonb not null default '{}',
	started_at timestamptz not null,
	completed_at timestamptz not null
);

create index if not exists messages_conversation_id_id_idx on messages(conversation_id,id);
create index if not exists conversations_updated_at_idx on conversations(updated_at desc);
create index if not exists inference_logs_completed_at_idx on inference_logs(completed_at desc);
create index if not exists inference_logs_provider_idx on inference_logs(provider);
create index if not exists inference_logs_status_idx on inference_logs(status);
