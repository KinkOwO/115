package main

import (
	"context"
	"dfolan/internal/database"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"
)

func (s *server) registerDashboardRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/mail/list", s.auth(s.handleGMMailList))
	mux.HandleFunc("/api/mail/send", s.auth(s.handleGMMailSend))
	mux.HandleFunc("/api/mail/revoke", s.auth(s.handleGMMailRevoke))
	mux.HandleFunc("/api/vault/send", s.auth(s.handleGMVaultSend))
}

func dashboardError(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
}

func dashboardRequest(w http.ResponseWriter, r *http.Request, method string, body any) bool {
	if r.Method != method {
		w.Header().Set("Allow", method)
		dashboardError(w, http.StatusMethodNotAllowed, errors.New("请求方法无效"))
		return false
	}
	if body != nil {
		raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			dashboardError(w, http.StatusBadRequest, err)
			return false
		}
		if err = json.Unmarshal(raw, body); err != nil {
			dashboardError(w, http.StatusBadRequest, errors.New("请求体不是合法 JSON"))
			return false
		}
	}
	return true
}

func (s *server) handleGMMailList(w http.ResponseWriter, r *http.Request) {
	if !dashboardRequest(w, r, http.MethodGet, nil) {
		return
	}
	var account int64
	if v := r.URL.Query().Get("account"); v != "" {
		var err error
		account, err = strconv.ParseInt(v, 10, 64)
		if err != nil {
			dashboardError(w, 400, errors.New("account 不是合法整数"))
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	mails, err := s.store.GMMails(ctx, account, r.URL.Query().Get("status"))
	if err != nil {
		dashboardError(w, 400, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "count": len(mails), "mails": mails})
}

func (s *server) handleGMMailSend(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Account   int64  `json:"to_account_id"`
		Character int64  `json:"to_character_id"`
		Template  int64  `json:"template"`
		Amount    int64  `json:"amount"`
		Title     string `json:"title"`
		Body      string `json:"body"`
	}
	if !dashboardRequest(w, r, http.MethodPost, &req) {
		return
	}
	if req.Title == "" {
		req.Title = "GM 邮件"
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	id, err := s.store.SendGMMail(ctx, req.Account, req.Character, req.Template, req.Amount, req.Title, req.Body)
	if err != nil {
		dashboardError(w, 400, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "mail_id": id, "to_account_id": req.Account, "to_character_id": req.Character, "template": req.Template, "amount": req.Amount, "message": "邮件已发送（管理台邮件系统）"})
}

func (s *server) handleGMMailRevoke(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID int64 `json:"id"`
	}
	if !dashboardRequest(w, r, http.MethodPost, &req) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	if err := s.store.RevokeGMMail(ctx, req.ID); err != nil {
		dashboardError(w, 400, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "mail_id": req.ID, "message": "邮件已撤销"})
}

func (s *server) handleGMVaultSend(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Account   int64 `json:"account"`
		Character int64 `json:"character"`
		Template  int64 `json:"template"`
		Amount    int64 `json:"amount"`
	}
	if !dashboardRequest(w, r, http.MethodPost, &req) {
		return
	}
	if req.Account <= 0 || req.Character <= 0 || req.Template <= 0 || req.Template > math.MaxUint32 || req.Amount <= 0 || req.Amount > math.MaxUint32 {
		dashboardError(w, 400, errors.New("account/character/template/amount 必须为正数，物品编号与数量不得超过 uint32"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	merged := false
	_, vault, err := s.store.CommitVaultMove(ctx, req.Account, req.Character, func(role database.Character, vault database.VaultState) (json.RawMessage, json.RawMessage, error) {
		items, m, e := addDashboardVaultItem(vault.Items, vault.Slots, req.Template, req.Amount)
		merged = m
		return role.State, items, e
	})
	if err != nil {
		dashboardError(w, 400, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "character_id": req.Character, "template": req.Template, "amount": req.Amount, "merged": merged, "slots": vault.Slots, "message": "已发放到个人仓库"})
}

// Retain unknown item fields and the dashboard's historical upper/lower case
// formats. Mutation runs under the existing character + primary vault locks.
func addDashboardVaultItem(raw json.RawMessage, slots uint16, template, amount int64) (json.RawMessage, bool, error) {
	var rows []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, false, err
	}
	number := func(row map[string]json.RawMessage, keys ...string) (int64, error) {
		for _, key := range keys {
			value := row[key]
			if len(value) == 0 {
				continue
			}
			var n int64
			if err := json.Unmarshal(value, &n); err != nil {
				return 0, err
			}
			if n != 0 {
				return n, nil
			}
		}
		return 0, nil
	}
	used := make(map[int64]bool, len(rows))
	for _, row := range rows {
		id, err := number(row, "Template", "template")
		if err != nil {
			return nil, false, err
		}
		if id == template {
			current, err := number(row, "Amount", "amount")
			if err != nil {
				return nil, false, err
			}
			if current < 0 || current > math.MaxUint32-amount {
				return nil, false, errors.New("仓库物品数量溢出")
			}
			row["Amount"] = json.RawMessage(strconv.FormatInt(current+amount, 10))
			out, err := json.Marshal(rows)
			return out, true, err
		}
		slot, err := number(row, "slot")
		if err != nil {
			return nil, false, err
		}
		used[slot] = true
	}
	for slot := int64(0); slot < int64(slots); slot++ {
		if used[slot] {
			continue
		}
		rows = append(rows, map[string]json.RawMessage{"slot": json.RawMessage(strconv.FormatInt(slot, 10)), "Template": json.RawMessage(strconv.FormatInt(template, 10)), "Amount": json.RawMessage(strconv.FormatInt(amount, 10))})
		out, err := json.Marshal(rows)
		return out, false, err
	}
	return nil, false, fmt.Errorf("仓库已满（%d 格）", slots)
}
