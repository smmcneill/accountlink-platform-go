package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"accountlink-platform-go/internal/app"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type (
	URLParamFunc func(r *http.Request, key string) string

	Handler struct {
		service  *app.AccountLinkService
		urlParam URLParamFunc
	}

	createAccountLinkRequest struct {
		UserID              string `json:"userId"`
		ExternalInstitution string `json:"externalInstitution"`
	}

	problemDetail struct {
		Title  string `json:"title"`
		Status int    `json:"status"`
		Detail string `json:"detail"`
	}
)

func NewHandler(service *app.AccountLinkService) *Handler {
	return &Handler{
		service:  service,
		urlParam: chi.URLParam,
	}
}

func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte("ok"))
}

func (h *Handler) GetAccountLink(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(h.urlParam(r, "id"))
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "Bad Request", "Invalid account link id")
		return
	}

	link, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, app.ErrNotFound) {
			writeProblem(w, http.StatusNotFound, "Not Found", err.Error())
			return
		}

		writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Request failed")

		return
	}

	writeJSON(w, http.StatusOK, link)
}

func (h *Handler) CreateAccountLink(w http.ResponseWriter, r *http.Request) {
	var req createAccountLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "Bad Request", "Malformed JSON")
		return
	}

	req.UserID = strings.TrimSpace(req.UserID)

	req.ExternalInstitution = strings.TrimSpace(req.ExternalInstitution)
	if req.UserID == "" || req.ExternalInstitution == "" {
		writeProblem(w, http.StatusBadRequest, "Bad Request", "userId and externalInstitution must not be blank")
		return
	}

	result, err := h.service.Create(r.Context(), strings.TrimSpace(r.Header.Get("Idempotency-Key")), req.UserID, req.ExternalInstitution)
	if err != nil {
		if errors.Is(err, app.ErrIdempotencyConflict) {
			writeProblem(w, http.StatusConflict, "Conflict", "Idempotency-Key was reused with a different request payload.")
			return
		}

		writeProblem(w, http.StatusInternalServerError, "Internal Server Error", "Request failed")

		return
	}

	location := "/account-links/" + result.Link.ID.String()
	w.Header().Set("Location", location)

	if result.Created {
		writeJSON(w, http.StatusCreated, result.Link)
		return
	}

	writeJSON(w, http.StatusOK, result.Link)
}

func writeProblem(w http.ResponseWriter, status int, title, detail string) {
	writeJSON(w, status, problemDetail{Title: title, Status: status, Detail: detail})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
