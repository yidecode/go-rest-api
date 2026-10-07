package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yidecode/go-rest-api/internal/config"
	"github.com/yidecode/go-rest-api/internal/store"
)

func testRouter() *gin.Engine {
	cfg := config.Config{JWTSecret: "test-secret", TokenTTL: time.Hour, RateLimitRPS: 1000}
	return NewRouter(store.NewMemoryStore(), cfg)
}

func doRequest(t *testing.T, r *gin.Engine, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func registerUser(t *testing.T, r *gin.Engine, username string) string {
	t.Helper()
	w := doRequest(t, r, http.MethodPost, "/api/v1/auth/register", "",
		map[string]string{"username": username, "password": "password123"})
	if w.Code != http.StatusCreated {
		t.Fatalf("register: got %d, body %s", w.Code, w.Body.String())
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Token == "" {
		t.Fatal("empty token")
	}
	return out.Token
}

func TestAuthFlow(t *testing.T) {
	r := testRouter()

	// duplicate registration is rejected
	_ = registerUser(t, r, "alice")
	w := doRequest(t, r, http.MethodPost, "/api/v1/auth/register", "",
		map[string]string{"username": "alice", "password": "password123"})
	if w.Code != http.StatusConflict {
		t.Fatalf("duplicate register: got %d, want 409", w.Code)
	}

	// login works
	w = doRequest(t, r, http.MethodPost, "/api/v1/auth/login", "",
		map[string]string{"username": "alice", "password": "password123"})
	if w.Code != http.StatusOK {
		t.Fatalf("login: got %d, body %s", w.Code, w.Body.String())
	}

	// wrong password is rejected
	w = doRequest(t, r, http.MethodPost, "/api/v1/auth/login", "",
		map[string]string{"username": "alice", "password": "wrongpass1"})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("bad login: got %d, want 401", w.Code)
	}

	// unauthenticated access is rejected
	w = doRequest(t, r, http.MethodGet, "/api/v1/me", "", nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("no token: got %d, want 401", w.Code)
	}
}

func TestTaskCRUD(t *testing.T) {
	r := testRouter()
	token := registerUser(t, r, "bob")

	// create
	w := doRequest(t, r, http.MethodPost, "/api/v1/tasks", token,
		map[string]string{"title": "Write docs", "description": "API docs", "status": "open"})
	if w.Code != http.StatusCreated {
		t.Fatalf("create: got %d, body %s", w.Code, w.Body.String())
	}
	var created store.Task
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	// list
	w = doRequest(t, r, http.MethodGet, "/api/v1/tasks", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: got %d", w.Code)
	}
	var listed struct {
		Tasks []store.Task `json:"tasks"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Tasks) != 1 {
		t.Fatalf("list: got %d tasks, want 1", len(listed.Tasks))
	}

	// get
	w = doRequest(t, r, http.MethodGet, "/api/v1/tasks/"+created.ID, token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get: got %d", w.Code)
	}

	// update
	w = doRequest(t, r, http.MethodPut, "/api/v1/tasks/"+created.ID, token,
		map[string]string{"title": "Write docs", "status": "done"})
	if w.Code != http.StatusOK {
		t.Fatalf("update: got %d, body %s", w.Code, w.Body.String())
	}
	var updated store.Task
	if err := json.Unmarshal(w.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Status != store.TaskDone {
		t.Fatalf("update: status %q, want done", updated.Status)
	}

	// delete
	w = doRequest(t, r, http.MethodDelete, "/api/v1/tasks/"+created.ID, token, nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete: got %d, want 204", w.Code)
	}

	// get after delete -> 404
	w = doRequest(t, r, http.MethodGet, "/api/v1/tasks/"+created.ID, token, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("get deleted: got %d, want 404", w.Code)
	}
}

func TestTasksAreUserScoped(t *testing.T) {
	r := testRouter()
	tokenA := registerUser(t, r, "carol")
	tokenB := registerUser(t, r, "dave")

	w := doRequest(t, r, http.MethodPost, "/api/v1/tasks", tokenA,
		map[string]string{"title": "Carol's task"})
	var created store.Task
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	// Dave must not see Carol's task
	w = doRequest(t, r, http.MethodGet, "/api/v1/tasks/"+created.ID, tokenB, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("cross-user read: got %d, want 404", w.Code)
	}
}
