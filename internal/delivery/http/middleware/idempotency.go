package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"

	"order-service/internal/delivery/http/response"
	"order-service/internal/idempotency"
)

func Idempotency(store idempotency.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("Idempotency-Key")
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}

			body, err := io.ReadAll(r.Body)
			if err != nil {
				response.WriteError(w, http.StatusBadRequest, "cannot read request body")
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			hash := hashBody(body)

			if rec, found, err := store.Get(r.Context(), key); err != nil {
				response.WriteError(w, http.StatusInternalServerError, "idempotency store error")
				return
			} else if found {
				if rec.RequestHash != hash {
					response.WriteError(w, http.StatusUnprocessableEntity,
						"idempotency key was already used with a different request body")
					return
				}
				writeStored(w, rec)
				return
			}

			reserved, err := store.Reserve(r.Context(), key, hash)
			if err != nil {
				response.WriteError(w, http.StatusInternalServerError, "idempotency store error")
				return
			}
			if !reserved {
				response.WriteError(w, http.StatusConflict,
					"a request with this idempotency key is already being processed")
				return
			}

			rec := httptest.NewRecorder()
			next.ServeHTTP(rec, r)

			_ = store.Save(r.Context(), key, rec.Code, rec.Body.Bytes())

			for k, values := range rec.Header() {
				for _, v := range values {
					w.Header().Add(k, v)
				}
			}
			w.WriteHeader(rec.Code)
			_, _ = w.Write(rec.Body.Bytes())
		})
	}
}

func writeStored(w http.ResponseWriter, rec *idempotency.Record) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Idempotent-Replayed", "true")
	w.WriteHeader(rec.StatusCode)
	_, _ = w.Write(rec.Body)
}

func hashBody(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
