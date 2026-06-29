CREATE TABLE IF NOT EXISTS documents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    slug TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    content_path TEXT NOT NULL UNIQUE,
    category_id INTEGER,
    source TEXT NOT NULL CHECK (source IN ('manual', 'import', 'agent')),
    status TEXT NOT NULL CHECK (status IN ('draft', 'published')),
    confidence REAL NOT NULL DEFAULT 0,
    word_count INTEGER NOT NULL DEFAULT 0,
    search_text TEXT NOT NULL DEFAULT '',
    cover_url TEXT NOT NULL DEFAULT '',
    author_id INTEGER,
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
