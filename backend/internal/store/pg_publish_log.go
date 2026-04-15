package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"ai-bot-chain/backend/internal/domain"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type PgPublishLog struct {
	db *sql.DB
}

func NewPgPublishLog(ctx context.Context, databaseURL string) (*PgPublishLog, error) {
	databaseURL = strings.TrimSpace(databaseURL)
	if databaseURL == "" {
		return nil, errors.New("missing DATABASE_URL")
	}
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}

	// Reasonable defaults for a local dev PG.
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(10 * time.Minute)

	p := &PgPublishLog{db: db}
	if err := p.pingAndMigrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return p, nil
}

func (p *PgPublishLog) Close() error {
	if p == nil || p.db == nil {
		return nil
	}
	return p.db.Close()
}

func (p *PgPublishLog) pingAndMigrate(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	if err := p.db.PingContext(ctx); err != nil {
		return err
	}

	// Minimal schema to store publish hashes locally.
	// We use tx_hash as the primary key because it's the stable query handle for explorers.
	const schema = `
CREATE TABLE IF NOT EXISTS training_publishes (
  tx_hash TEXT PRIMARY KEY,
  bot_id TEXT NOT NULL,
  root_hash TEXT,
  storage_reference TEXT,
  sample_count INTEGER NOT NULL DEFAULT 0,
  mode TEXT,
  explorer_tx_url TEXT,
  indexer_rpc TEXT,
  evm_rpc TEXT,
  tx_mined BOOLEAN NOT NULL DEFAULT FALSE,
  tx_success BOOLEAN NOT NULL DEFAULT FALSE,
  upload_pending BOOLEAN NOT NULL DEFAULT FALSE,
  upload_completed BOOLEAN NOT NULL DEFAULT FALSE,
  published_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS training_publishes_bot_id_idx ON training_publishes (bot_id);
`
	_, err := p.db.ExecContext(ctx, schema)
	return err
}

func (p *PgPublishLog) SaveTrainingPublish(ctx context.Context, r domain.PublishResult) error {
	if p == nil || p.db == nil {
		return errors.New("pgsql publish log not initialized")
	}
	r = normalizePublishedAt(r)

	txHash := strings.TrimSpace(r.TxHash)
	if txHash == "" {
		return errors.New("txHash is empty (cannot persist publish)")
	}

	const q = `
INSERT INTO training_publishes (
  tx_hash, bot_id, root_hash, storage_reference, sample_count, mode,
  explorer_tx_url, indexer_rpc, evm_rpc, tx_mined, tx_success,
  upload_pending, upload_completed, published_at
) VALUES (
  $1,$2,$3,$4,$5,$6,
  $7,$8,$9,$10,$11,
  $12,$13,$14
)
ON CONFLICT (tx_hash) DO UPDATE SET
  bot_id = EXCLUDED.bot_id,
  root_hash = EXCLUDED.root_hash,
  storage_reference = EXCLUDED.storage_reference,
  sample_count = EXCLUDED.sample_count,
  mode = EXCLUDED.mode,
  explorer_tx_url = EXCLUDED.explorer_tx_url,
  indexer_rpc = EXCLUDED.indexer_rpc,
  evm_rpc = EXCLUDED.evm_rpc,
  tx_mined = EXCLUDED.tx_mined,
  tx_success = EXCLUDED.tx_success,
  upload_pending = EXCLUDED.upload_pending,
  upload_completed = EXCLUDED.upload_completed,
  published_at = EXCLUDED.published_at
`
	_, err := p.db.ExecContext(ctx, q,
		txHash,
		strings.TrimSpace(r.BotID),
		nullIfEmpty(r.RootHash),
		nullIfEmpty(r.StorageReference),
		r.SampleCount,
		nullIfEmpty(r.Mode),
		nullIfEmpty(r.ExplorerTxURL),
		nullIfEmpty(r.IndexerRPC),
		nullIfEmpty(r.EvmRPC),
		r.TxMined,
		r.TxSuccess,
		r.UploadPending,
		r.UploadCompleted,
		r.PublishedAt,
	)
	if err != nil {
		return fmt.Errorf("persist publish tx hash: %w", err)
	}
	return nil
}

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return strings.TrimSpace(s)
}
