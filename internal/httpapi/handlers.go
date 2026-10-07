package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/yidecode/go-rest-api/internal/auth"
	"github.com/yidecode/go-rest-api/internal/config"
	"github.com/yidecode/go-rest-api/internal/store"
)

// Handler bundles dependencies for the HTTP handlers.
type Handler struct {
	store store.Store
	cfg   config.Config
}

// NewHandler builds a Handler.
func NewHandler(s store.Store, cfg config.Config) *Handler {
	return &Handler{store: s, cfg: cfg}
}

type credentials struct {
	Username string `json:"username" binding:"required,min=3,max=32"`
	Password string `json:"password" binding:"required,min=8,max=72"`
}

func tokenResponse(u store.User, cfg config.Config) (int, gin.H, error) {
	tok, err := auth.Issue(cfg.JWTSecret, u.ID, cfg.TokenTTL)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}
	return http.StatusOK, gin.H{
		"token": tok,
		"user":  gin.H{"id": u.ID, "username": u.Username},
	}, nil
}

// Register creates a new user account.
func (h *Handler) Register(c *gin.Context) {
	var in credentials
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot hash password"})
		return
	}
	u, err := h.store.CreateUser(c.Request.Context(), in.Username, string(hash))
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "username taken"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot create user"})
		return
	}
	status, body, err := tokenResponse(u, h.cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot issue token"})
		return
	}
	c.JSON(http.StatusCreated, body)
	_ = status
}

// Login verifies credentials and returns a token.
func (h *Handler) Login(c *gin.Context) {
	var in credentials
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	u, err := h.store.GetUserByUsername(c.Request.Context(), in.Username)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(in.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	status, body, err := tokenResponse(u, h.cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot issue token"})
		return
	}
	c.JSON(status, body)
}

// Me returns the authenticated user's ID.
func (h *Handler) Me(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"user_id": currentUserID(c)})
}

// Health is an unauthenticated liveness probe.
func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

type taskPayload struct {
	Title       string           `json:"title" binding:"required,max=200"`
	Description string           `json:"description" binding:"max=2000"`
	Status      store.TaskStatus `json:"status"`
}

func toTaskInput(p taskPayload) (store.TaskInput, error) {
	if p.Status != "" && !p.Status.Valid() {
		return store.TaskInput{}, errors.New("invalid status: must be open, in_progress or done")
	}
	return store.TaskInput{Title: p.Title, Description: p.Description, Status: p.Status}, nil
}

// CreateTask creates a task for the authenticated user.
func (h *Handler) CreateTask(c *gin.Context) {
	var p taskPayload
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	in, err := toTaskInput(p)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	t, err := h.store.CreateTask(c.Request.Context(), currentUserID(c), in)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot create task"})
		return
	}
	c.JSON(http.StatusCreated, t)
}

// ListTasks lists the authenticated user's tasks with optional filters.
func (h *Handler) ListTasks(c *gin.Context) {
	var f store.TaskFilter
	if s := c.Query("status"); s != "" {
		f.Status = store.TaskStatus(s)
		if !f.Status.Valid() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status filter"})
			return
		}
	}
	f.Query = c.Query("q")
	if v := c.Query("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
			return
		}
		f.Limit = n
	}
	if v := c.Query("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid offset"})
			return
		}
		f.Offset = n
	}
	tasks, err := h.store.ListTasks(c.Request.Context(), currentUserID(c), f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot list tasks"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"tasks": tasks})
}

// GetTask returns a single task.
func (h *Handler) GetTask(c *gin.Context) {
	t, err := h.store.GetTask(c.Request.Context(), currentUserID(c), c.Param("id"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot get task"})
		return
	}
	c.JSON(http.StatusOK, t)
}

// UpdateTask replaces a task.
func (h *Handler) UpdateTask(c *gin.Context) {
	var p taskPayload
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	in, err := toTaskInput(p)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	t, err := h.store.UpdateTask(c.Request.Context(), currentUserID(c), c.Param("id"), in)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot update task"})
		return
	}
	c.JSON(http.StatusOK, t)
}

// DeleteTask removes a task.
func (h *Handler) DeleteTask(c *gin.Context) {
	if err := h.store.DeleteTask(c.Request.Context(), currentUserID(c), c.Param("id")); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot delete task"})
		return
	}
	c.Status(http.StatusNoContent)
}
