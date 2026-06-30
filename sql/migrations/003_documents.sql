CREATE TABLE IF NOT EXISTS documents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    slug TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    category_id INTEGER,
    source TEXT NOT NULL CHECK (source IN ('manual', 'import', 'agent')),
    status TEXT NOT NULL CHECK (status IN ('draft', 'published')),
    confidence REAL NOT NULL DEFAULT 0,
    word_count INTEGER NOT NULL DEFAULT 0,
    search_text TEXT NOT NULL DEFAULT '',
    cover_url TEXT NOT NULL DEFAULT '',
    author_id INTEGER,
    current_version INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    published_at TEXT,
    FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE RESTRICT,
    FOREIGN KEY (author_id) REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_documents_status_updated_at
ON documents (status, updated_at);

CREATE INDEX IF NOT EXISTS idx_documents_category_status
ON documents (category_id, status);

CREATE INDEX IF NOT EXISTS idx_documents_slug
ON documents (slug);

CREATE TABLE IF NOT EXISTS document_blocks (
    block_id TEXT PRIMARY KEY,
    document_id INTEGER NOT NULL,
    parent_id TEXT NOT NULL DEFAULT '',
    position_key TEXT NOT NULL,
    type TEXT NOT NULL,
    content_json TEXT NOT NULL,
    text_content TEXT NOT NULL DEFAULT '',
    version INTEGER NOT NULL DEFAULT 1,
    updated_by INTEGER,
    updated_at TEXT NOT NULL,
    FOREIGN KEY (document_id) REFERENCES documents(id) ON DELETE CASCADE,
    FOREIGN KEY (updated_by) REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_document_blocks_document_position
ON document_blocks (document_id, position_key);

CREATE TABLE IF NOT EXISTS document_ops (
    op_id TEXT PRIMARY KEY,
    document_id INTEGER NOT NULL,
    actor_id INTEGER,
    base_document_version INTEGER NOT NULL,
    block_id TEXT NOT NULL,
    op_type TEXT NOT NULL,
    payload_json TEXT NOT NULL,
    document_version INTEGER NOT NULL,
    created_at TEXT NOT NULL,
    FOREIGN KEY (document_id) REFERENCES documents(id) ON DELETE CASCADE,
    FOREIGN KEY (actor_id) REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_document_ops_document_version
ON document_ops (document_id, document_version);

CREATE TABLE IF NOT EXISTS document_revisions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    document_id INTEGER NOT NULL,
    version INTEGER NOT NULL,
    snapshot_json TEXT NOT NULL,
    content_text TEXT NOT NULL,
    created_by INTEGER,
    created_at TEXT NOT NULL,
    UNIQUE (document_id, version),
    FOREIGN KEY (document_id) REFERENCES documents(id) ON DELETE CASCADE,
    FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_document_revisions_document_version
ON document_revisions (document_id, version);

CREATE TABLE IF NOT EXISTS document_tags (
    document_id INTEGER NOT NULL,
    tag_id INTEGER NOT NULL,
    PRIMARY KEY (document_id, tag_id),
    FOREIGN KEY (document_id) REFERENCES documents(id) ON DELETE CASCADE,
    FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_document_tags_tag_document
ON document_tags (tag_id, document_id);

CREATE VIRTUAL TABLE IF NOT EXISTS documents_fts USING fts5(
    title,
    summary,
    search_text,
    content='documents',
    content_rowid='id',
    tokenize='unicode61'
);

INSERT INTO documents_fts(rowid, title, summary, search_text)
SELECT id, title, COALESCE(summary, ''), COALESCE(search_text, '')
FROM documents;

CREATE TRIGGER IF NOT EXISTS documents_ai AFTER INSERT ON documents BEGIN
    INSERT INTO documents_fts(rowid, title, summary, search_text)
    VALUES (new.id, new.title, COALESCE(new.summary, ''), COALESCE(new.search_text, ''));
END;

CREATE TRIGGER IF NOT EXISTS documents_ad AFTER DELETE ON documents BEGIN
    INSERT INTO documents_fts(documents_fts, rowid, title, summary, search_text)
    VALUES ('delete', old.id, old.title, COALESCE(old.summary, ''), COALESCE(old.search_text, ''));
END;

CREATE TRIGGER IF NOT EXISTS documents_au AFTER UPDATE ON documents BEGIN
    INSERT INTO documents_fts(documents_fts, rowid, title, summary, search_text)
    VALUES ('delete', old.id, old.title, COALESCE(old.summary, ''), COALESCE(old.search_text, ''));
    INSERT INTO documents_fts(rowid, title, summary, search_text)
    VALUES (new.id, new.title, COALESCE(new.summary, ''), COALESCE(new.search_text, ''));
END;
