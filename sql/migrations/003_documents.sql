CREATE TABLE IF NOT EXISTS documents (
    id BIGSERIAL PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    category_id BIGINT REFERENCES categories(id) ON DELETE RESTRICT,
    source TEXT NOT NULL CHECK (source IN ('manual', 'import', 'agent')),
    status TEXT NOT NULL CHECK (status IN ('draft', 'published')),
    confidence DOUBLE PRECISION NOT NULL DEFAULT 0,
    word_count INTEGER NOT NULL DEFAULT 0,
    search_text TEXT NOT NULL DEFAULT '',
    search_vector TSVECTOR GENERATED ALWAYS AS (
        to_tsvector('simple', coalesce(title, '') || ' ' || coalesce(summary, '') || ' ' || coalesce(search_text, ''))
    ) STORED,
    cover_url TEXT NOT NULL DEFAULT '',
    author_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    current_version BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    published_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_documents_status_updated_at
ON documents (status, updated_at);

CREATE INDEX IF NOT EXISTS idx_documents_category_status
ON documents (category_id, status);

CREATE INDEX IF NOT EXISTS idx_documents_slug
ON documents (slug);

CREATE INDEX IF NOT EXISTS idx_documents_search_vector
ON documents USING GIN (search_vector);

CREATE TABLE IF NOT EXISTS document_blocks (
    block_id TEXT PRIMARY KEY,
    document_id BIGINT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    parent_id TEXT NOT NULL DEFAULT '',
    position_key TEXT NOT NULL,
    type TEXT NOT NULL,
    content_json JSONB NOT NULL,
    text_content TEXT NOT NULL DEFAULT '',
    version BIGINT NOT NULL DEFAULT 1,
    updated_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_document_blocks_document_position
ON document_blocks (document_id, position_key);

CREATE TABLE IF NOT EXISTS document_ops (
    op_id TEXT PRIMARY KEY,
    document_id BIGINT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    actor_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    base_document_version BIGINT NOT NULL,
    block_id TEXT NOT NULL,
    op_type TEXT NOT NULL,
    payload_json JSONB NOT NULL,
    document_version BIGINT NOT NULL,
    block_version BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_document_ops_document_version
ON document_ops (document_id, document_version);

CREATE TABLE IF NOT EXISTS document_revisions (
    id BIGSERIAL PRIMARY KEY,
    document_id BIGINT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    version BIGINT NOT NULL,
    snapshot_json JSONB NOT NULL,
    content_text TEXT NOT NULL,
    created_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL,
    UNIQUE (document_id, version)
);

CREATE INDEX IF NOT EXISTS idx_document_revisions_document_version
ON document_revisions (document_id, version);

CREATE TABLE IF NOT EXISTS document_tags (
    document_id BIGINT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    tag_id BIGINT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (document_id, tag_id)
);

CREATE INDEX IF NOT EXISTS idx_document_tags_tag_document
ON document_tags (tag_id, document_id);
