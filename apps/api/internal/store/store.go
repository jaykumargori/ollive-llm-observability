package store

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"ollive-llm-observability/packages/sdk"
)

type Store struct{ db *pgxpool.Pool }

type Conversation struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Provider  string    `json:"provider"`
	Model     string    `json:"model"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Message struct {
	ID        int64     `json:"id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

func New(db *pgxpool.Pool) *Store { return &Store{db: db} }

func (s *Store) CreateConversation(ctx context.Context, provider, model, title string) (string, error) {
	var id string
	err := s.db.QueryRow(ctx, `insert into conversations(title,provider,model) values($1,$2,$3) returning id`, title, provider, model).Scan(&id)
	return id, err
}

func (s *Store) ListConversations(ctx context.Context) ([]Conversation, error) {
	rows, err := s.db.Query(ctx, `select id,title,provider,model,updated_at from conversations order by updated_at desc limit 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Conversation
	for rows.Next() {
		var c Conversation
		if err := rows.Scan(&c.ID, &c.Title, &c.Provider, &c.Model, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) DeleteConversation(ctx context.Context, conversationID string) error {
	_, err := s.db.Exec(ctx, `delete from conversations where id=$1`, conversationID)
	return err
}

func (s *Store) Messages(ctx context.Context, conversationID string) ([]Message, error) {
	rows, err := s.db.Query(ctx, `select id,role,content,created_at from messages where conversation_id=$1 order by id asc`, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.Role, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) AddMessage(ctx context.Context, conversationID, role, content string) error {
	_, err := s.db.Exec(ctx, `insert into messages(conversation_id,role,content) values($1,$2,$3)`, conversationID, role, content)
	if err == nil && role == "user" {
		title := titleFromPrompt(content)
		_, _ = s.db.Exec(ctx, `
			update conversations
			set title=$2, updated_at=now()
			where id=$1 and title in ('New conversation','Inference trace')`,
			conversationID, title)
	}
	return err
}

func (s *Store) Touch(ctx context.Context, conversationID string) {
	_, _ = s.db.Exec(ctx, `update conversations set updated_at=now() where id=$1`, conversationID)
}

func titleFromPrompt(content string) string {
	title := strings.Join(strings.Fields(content), " ")
	if title == "" {
		return "New conversation"
	}
	runes := []rune(title)
	if len(runes) > 56 {
		return string(runes[:56]) + "..."
	}
	return title
}

func (s *Store) RecentContext(ctx context.Context, conversationID string, limit int) ([]sdk.Message, error) {
	rows, err := s.db.Query(ctx, `select role,content from messages where conversation_id=$1 order by id desc limit $2`, conversationID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rev []sdk.Message
	for rows.Next() {
		var m sdk.Message
		if err := rows.Scan(&m.Role, &m.Content); err != nil {
			return nil, err
		}
		rev = append(rev, m)
	}
	out := make([]sdk.Message, len(rev))
	for i := range rev {
		out[len(rev)-1-i] = rev[i]
	}
	return out, rows.Err()
}

func (s *Store) Metrics(ctx context.Context) (map[string]any, error) {
	row := s.db.QueryRow(ctx, `
		select
			coalesce(avg(latency_ms),0)::bigint,
			count(*) filter (where completed_at > now() - interval '1 hour'),
			coalesce(100.0 * count(*) filter (where status <> 'ok') / nullif(count(*),0),0),
			coalesce(sum(input_tokens + output_tokens),0)
		from inference_logs`)
	var avgLatency, throughput, tokens int64
	var errorRate float64
	if err := row.Scan(&avgLatency, &throughput, &errorRate, &tokens); err != nil {
		return nil, err
	}
	prows, err := s.db.Query(ctx, `select provider,count(*) from inference_logs group by provider order by count(*) desc`)
	if err != nil {
		return nil, err
	}
	defer prows.Close()
	providers := map[string]int64{}
	for prows.Next() {
		var p string
		var n int64
		if err := prows.Scan(&p, &n); err != nil {
			return nil, err
		}
		providers[p] = n
	}
	return map[string]any{"avg_latency_ms": avgLatency, "throughput_1h": throughput, "error_rate": errorRate, "tokens": tokens, "providers": providers}, nil
}
