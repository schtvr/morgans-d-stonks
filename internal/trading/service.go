package trading

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Service coordinates policy, idempotency, and persistence.
type Service struct {
	Repo   Repository
	Policy Policy
	Clock  func() time.Time
}

// NewService builds a service with sane defaults.
func NewService(repo Repository, policy Policy) *Service {
	return &Service{
		Repo:   repo,
		Policy: policy,
		Clock:  time.Now,
	}
}

// Validate evaluates the request without mutating state.
func (s *Service) Validate(ctx context.Context, req OrderRequest) (RiskDecision, error) {
	_ = ctx
	if err := Validate(req); err != nil {
		return RiskDecision{}, err
	}
	return s.Policy.Evaluate(PolicyContext{Provider: req.Provider, AvailableCash: req.AvailableCash}, req), nil
}

// Create creates or replays an order request.
func (s *Service) Create(ctx context.Context, req OrderRequest) (*OrderResponse, error) {
	if err := Validate(req); err != nil {
		return nil, err
	}
	if req.IdempotencyKey == "" {
		return nil, fmt.Errorf("idempotencyKey is required")
	}
	if req.RequestHash == "" {
		req.RequestHash = HashRequest(req)
	}
	if existing, err := s.Repo.GetOrderByIdempotencyKey(ctx, req.IdempotencyKey); err == nil {
		if existing.RequestHash != req.RequestHash {
			return nil, fmt.Errorf("idempotency key reuse with different request hash")
		}
		var resp OrderResponse
		if len(existing.ResponseJSON) > 0 {
			_ = json.Unmarshal(existing.ResponseJSON, &resp)
			return &resp, nil
		}
		return &OrderResponse{Order: *existing, Decision: s.Policy.Evaluate(PolicyContext{Provider: req.Provider, AvailableCash: req.AvailableCash}, req)}, nil
	} else if !errors.Is(err, ErrOrderNotFound) {
		return nil, err
	}

	decision := s.Policy.Evaluate(PolicyContext{Provider: req.Provider, AvailableCash: req.AvailableCash}, req)
	now := s.now()
	order := Order{
		ID:             uuid.NewString(),
		Symbol:         req.Symbol,
		Side:           req.Side,
		Quantity:       req.Quantity,
		LimitPrice:     req.LimitPrice,
		Notional:       decision.Notional,
		Status:         OrderStatusNew,
		IdempotencyKey: req.IdempotencyKey,
		RequestHash:    req.RequestHash,
		Provider:       req.Provider,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if decision.Allowed {
		order.Status = OrderStatusAccepted
	} else {
		order.Status = OrderStatusRejected
		order.Reason = joinReasonCodes(decision.ReasonCodes)
	}
	resp := OrderResponse{Order: order, Decision: decision}
	payload, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}
	order.ResponseJSON = payload
	if err := s.Repo.CreateOrder(ctx, order); err != nil {
		return nil, err
	}
	if err := s.Repo.SaveResponseJSON(ctx, order.ID, payload); err != nil {
		return nil, err
	}
	eventType := "accept"
	if !decision.Allowed {
		eventType = "reject"
	}
	next, _ := Transition(OrderStatusNew, eventType)
	if err := s.Repo.AppendOrderEvent(ctx, OrderEvent{
		OrderID:    order.ID,
		EventType:  eventType,
		FromStatus: OrderStatusNew,
		ToStatus:   next,
		Reason:     order.Reason,
		CreatedAt:  now,
	}); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Get returns an order by ID.
func (s *Service) Get(ctx context.Context, id string) (*Order, error) {
	return s.Repo.GetOrder(ctx, id)
}

// Cancel marks an open order canceled.
func (s *Service) Cancel(ctx context.Context, id string) (*Order, error) {
	order, err := s.Repo.GetOrder(ctx, id)
	if err != nil {
		return nil, err
	}
	if !IsOpen(order.Status) {
		return nil, fmt.Errorf("order %s is not open", id)
	}
	next, err := Transition(order.Status, "cancel")
	if err != nil {
		return nil, err
	}
	now := s.now()
	if err := s.Repo.UpdateOrderStatus(ctx, id, next, "canceled by request", now); err != nil {
		return nil, err
	}
	if err := s.Repo.AppendOrderEvent(ctx, OrderEvent{
		OrderID:    id,
		EventType:  "cancel",
		FromStatus: order.Status,
		ToStatus:   next,
		Reason:     "canceled by request",
		CreatedAt:  now,
	}); err != nil {
		return nil, err
	}
	order.Status = next
	order.Reason = "canceled by request"
	order.UpdatedAt = now
	return order, nil
}

// HashRequest returns a deterministic hash for idempotency validation.
func HashRequest(req OrderRequest) string {
	b, _ := json.Marshal(req)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func (s *Service) now() time.Time {
	if s.Clock != nil {
		return s.Clock().UTC()
	}
	return time.Now().UTC()
}

func joinReasonCodes(codes []string) string {
	if len(codes) == 0 {
		return ""
	}
	out := codes[0]
	for i := 1; i < len(codes); i++ {
		out += "," + codes[i]
	}
	return out
}

// ErrOrderNotFound is returned when the repository lookup misses.
var ErrOrderNotFound = errors.New("trading order not found")
