package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// --- Models ---
type Post struct {
	ID          int             `json:"id"`
	Title       string          `json:"title"`
	Slug        string          `json:"slug"`
	Body        json.RawMessage `json:"body"` // JSON block content from editors like TipTap/Editor.js
	Status      string          `json:"status"` // "draft" or "published"
	AuthorID    int             `json:"author_id"`
	PublishedAt *time.Time      `json:"published_at,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

type Application struct {
	db *sql.DB
}

func main() {
	// Initialize SQLite Database
	db, err := sql.Open("sqlite3", "./cms.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := initSchema(db); err != nil {
		log.Fatal("Failed to initialize schema:", err)
	}

	app := &Application{db: db}

	// Router setup using Standard Library net/http (Go 1.22+)
	mux := http.NewServeMux()

	// Public API Endpoints
	mux.HandleFunc("GET /api/posts", app.listPublishedPosts)
	mux.HandleFunc("GET /api/posts/{slug}", app.getPostBySlug)

	// Admin/Protected Endpoints (Wrap with Auth Middleware)
	mux.HandleFunc("POST /api/admin/posts", app.requireAuth(app.createPost))
	mux.HandleFunc("PUT /api/admin/posts/{id}", app.requireAuth(app.updatePost))
	mux.HandleFunc("DELETE /api/admin/posts/{id}", app.requireAuth(app.deletePost))

	// Static Admin Dashboard UI (Embedded or Served directly)
	mux.Handle("/", http.FileServer(http.Dir("./public")))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("CMS Server running on http://localhost:%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}

// --- Database Schema ---
func initSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		email TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'author'
	);

	CREATE TABLE IF NOT EXISTS posts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		slug TEXT UNIQUE NOT NULL,
		body TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'draft',
		author_id INTEGER NOT NULL,
		published_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(author_id) REFERENCES users(id)
	);

	CREATE TABLE IF NOT EXISTS post_revisions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		post_id INTEGER NOT NULL,
		body TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(post_id) REFERENCES posts(id) ON DELETE CASCADE
	);

	-- Built-in SQLite Full-Text Search Table
	CREATE VIRTUAL TABLE IF NOT EXISTS posts_fts USING fts5(title, body, content='posts', content_rowid='id');
	`
	_, err := db.Exec(schema)
	return err
}

// --- Handlers ---
func (app *Application) listPublishedPosts(w http.ResponseWriter, r *http.Request) {
	rows, err := app.db.Query("SELECT id, title, slug, body, status, author_id, published_at, created_at, updated_at FROM posts WHERE status = 'published' ORDER BY published_at DESC")
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	posts := []Post{}
	for rows.Next() {
		var p Post
		if err := rows.Scan(&p.ID, &p.Title, &p.Slug, &p.Body, &p.Status, &p.AuthorID, &p.PublishedAt, &p.CreatedAt, &p.UpdatedAt); err != nil {
			http.Error(w, "Error scanning data", http.StatusInternalServerError)
			return
		}
		posts = append(posts, p)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=60, s-maxage=3600")
	json.NewEncoder(w).Encode(posts)
}

func (app *Application) getPostBySlug(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")

	var p Post
	err := app.db.QueryRow("SELECT id, title, slug, body, status, author_id, published_at, created_at, updated_at FROM posts WHERE slug = ? AND status = 'published'", slug).
		Scan(&p.ID, &p.Title, &p.Slug, &p.Body, &p.Status, &p.AuthorID, &p.PublishedAt, &p.CreatedAt, &p.UpdatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Post not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=300")
	json.NewEncoder(w).Encode(p)
}

func (app *Application) createPost(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title  string          `json:"title"`
		Slug   string          `json:"slug"`
		Body   json.RawMessage `json:"body"`
		Status string          `json:"status"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	var publishedAt *time.Time
	if input.Status == "published" {
		now := time.Now()
		publishedAt = &now
	}

	// Simple hardcoded author_id for demo (typically extracted from session context)
	authorID := 1

	tx, err := app.db.Begin()
	if err != nil {
		http.Error(w, "Transaction error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	res, err := tx.Exec(
		"INSERT INTO posts (title, slug, body, status, author_id, published_at) VALUES (?, ?, ?, ?, ?, ?)",
		input.Title, input.Slug, string(input.Body), input.Status, authorID, publishedAt,
	)
	if err != nil {
		http.Error(w, "Could not insert post", http.StatusInternalServerError)
		return
	}

	postID, _ := res.LastInsertId()

	// Create initial revision record
	_, _ = tx.Exec("INSERT INTO post_revisions (post_id, body) VALUES (?, ?)", postID, string(input.Body))

	// Keep FTS Index synced
	_, _ = tx.Exec("INSERT INTO posts_fts(rowid, title, body) VALUES (?, ?, ?)", postID, input.Title, string(input.Body))

	if err := tx.Commit(); err != nil {
		http.Error(w, "Commit error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, `{"id": %d, "message": "Post created successfully"}`, postID)
}

func (app *Application) updatePost(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var input struct {
		Title  string          `json:"title"`
		Slug   string          `json:"slug"`
		Body   json.RawMessage `json:"body"`
		Status string          `json:"status"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	tx, err := app.db.Begin()
	if err != nil {
		http.Error(w, "Transaction error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		"UPDATE posts SET title = ?, slug = ?, body = ?, status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		input.Title, input.Slug, string(input.Body), input.Status, id,
	)
	if err != nil {
		http.Error(w, "Failed to update post", http.StatusInternalServerError)
		return
	}

	// Add new revision snapshot
	_, _ = tx.Exec("INSERT INTO post_revisions (post_id, body) VALUES (?, ?)", id, string(input.Body))

	if err := tx.Commit(); err != nil {
		http.Error(w, "Commit error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message": "Post updated"}`))
}

func (app *Application) deletePost(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_, err := app.db.Exec("DELETE FROM posts WHERE id = ?", id)
	if err != nil {
		http.Error(w, "Failed to delete post", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Middleware ---
func (app *Application) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Replace this with actual Cookie session or Bearer JWT check
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	}
}