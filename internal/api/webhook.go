// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/quad/quad/internal/provisioning"
	"github.com/quad/quad/pkg/adapter"
)

// pushPayload is the subset of a host push event Quad needs. GitHub puts the org
// in owner.login; Gitea/Forgejo use owner.username — we accept either.
type pushPayload struct {
	After      string `json:"after"`
	Repository struct {
		Name  string `json:"name"`
		Owner struct {
			Login    string `json:"login"`
			Username string `json:"username"`
		} `json:"owner"`
	} `json:"repository"`
}

func (p pushPayload) namespace() string {
	if p.Repository.Owner.Login != "" {
		return p.Repository.Owner.Login
	}
	return p.Repository.Owner.Username
}

// handleWebhook receives a host push delivery, verifies its HMAC signature
// against the per-host secret, maps the repo to a submission, and (if a grader is
// configured) enqueues a regrade keyed to the head commit so each push grades once.
//
// It is public by necessity — the Git host calls it — so the HMAC signature is the
// only trust boundary. No secret configured for a host means we cannot trust any
// delivery for it, so we 404.
func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	host := adapter.Host(r.PathValue("host"))
	secret := s.webhookSecrets[host]
	if secret == "" {
		// No secret → unverifiable → we do not accept deliveries for this host.
		httpError(w, http.StatusNotFound, "no webhook secret configured for this host")
		return
	}

	// Read the raw body once: HMAC must be computed over the exact bytes the host
	// signed, so we cannot let json.Decode consume the stream first.
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequestBody))
	if err != nil {
		httpError(w, http.StatusRequestEntityTooLarge, "request body too large")
		return
	}

	if !verifyWebhookSignature(host, r.Header, body, secret) {
		httpError(w, http.StatusUnauthorized, "invalid or missing signature")
		return
	}

	var payload pushPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		httpError(w, http.StatusBadRequest, "malformed payload")
		return
	}

	// Non-push deliveries (e.g. ping) and branch deletions carry no actionable head
	// commit. Acknowledge them without doing anything.
	ns, name, sha := payload.namespace(), payload.Repository.Name, payload.After
	if name == "" || sha == "" || isZeroSHA(sha) {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	sub, err := s.store.FindSubmissionByRepo(r.Context(), host, ns, name)
	if err != nil {
		// A delivery for a repo we don't track is not an error.
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Record the new head so the student view reflects the push immediately.
	now := time.Now()
	sub.LatestCommit = sha
	sub.LastActivityAt = &now
	if err := s.store.UpdateSubmission(r.Context(), sub); err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if !s.graderConfigured {
		// Don't enqueue jobs that would only fail; the push is still recorded.
		log.Printf("webhook: push for submission %s recorded; no grader configured, skipping regrade", sub.ID)
		w.WriteHeader(http.StatusOK)
		return
	}

	// Idempotency key includes the sha so the same push grades once but a new
	// commit triggers a fresh regrade.
	idem := "grade:webhook:" + sub.ID + ":" + sha
	if err := s.queue.Enqueue(r.Context(), provisioning.JobGrade, sub.ID, idem); err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// verifyWebhookSignature checks the delivery's HMAC-SHA256 over body using a
// constant-time compare. GitHub sends X-Hub-Signature-256: sha256=<hex>;
// Gitea/Forgejo send X-Gitea-Signature / X-Forgejo-Signature: <hex>.
func verifyWebhookSignature(host adapter.Host, h http.Header, body []byte, secret string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	want := mac.Sum(nil)

	var got []byte
	if host == adapter.HostGitHub {
		sig := h.Get("X-Hub-Signature-256")
		hexPart, ok := strings.CutPrefix(sig, "sha256=")
		if !ok {
			return false
		}
		got = decodeHex(hexPart)
	} else {
		sig := h.Get("X-Gitea-Signature")
		if sig == "" {
			sig = h.Get("X-Forgejo-Signature")
		}
		got = decodeHex(sig)
	}
	if len(got) == 0 {
		return false
	}
	return hmac.Equal(got, want)
}

func decodeHex(s string) []byte {
	b, err := hex.DecodeString(strings.TrimSpace(s))
	if err != nil {
		return nil
	}
	return b
}

// isZeroSHA reports whether sha is the all-zero SHA a host sends for a branch
// deletion (no head commit to grade).
func isZeroSHA(sha string) bool {
	for _, c := range sha {
		if c != '0' {
			return false
		}
	}
	return true
}
