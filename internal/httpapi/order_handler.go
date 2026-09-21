package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/order"
)

// OrderHandler depends on order.Service — the one place item validation,
// server-side pricing, and coupon checking come together.
type OrderHandler struct {
	Service order.Service
}

type orderItemRequest struct {
	ProductID string `json:"productId"`
	Quantity  int    `json:"quantity"`
}

type placeOrderRequest struct {
	CouponCode string             `json:"couponCode,omitempty"`
	Items      []orderItemRequest `json:"items"`
}

type orderItemResponse struct {
	ProductID string `json:"productId"`
	Quantity  int    `json:"quantity"`
}

type orderResponse struct {
	ID       string              `json:"id"`
	Items    []orderItemResponse `json:"items"`
	Products []productResponse   `json:"products"`
}

func toOrderResponse(o order.Order) orderResponse {
	items := make([]orderItemResponse, len(o.Items))
	for i, it := range o.Items {
		items[i] = orderItemResponse{ProductID: it.ProductID, Quantity: it.Quantity}
	}
	products := make([]productResponse, len(o.Products))
	for i, p := range o.Products {
		products[i] = toProductResponse(p)
	}
	return orderResponse{ID: o.ID, Items: items, Products: products}
}

// Create handles POST /order. Validation failures (empty items, bad
// quantity, unknown product, invalid coupon) all map to 422 per the
// spec's "Validation exception" response — a malformed request body is
// the only case that's a 400.
func (h *OrderHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req placeOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_body", "request body must be valid JSON")
		return
	}

	items := make([]order.Item, len(req.Items))
	for i, it := range req.Items {
		items[i] = order.Item{ProductID: it.ProductID, Quantity: it.Quantity}
	}

	created, err := h.Service.PlaceOrder(r.Context(), order.CreateOrderRequest{
		Items:      items,
		CouponCode: req.CouponCode,
	})
	if err != nil {
		switch {
		case errors.Is(err, order.ErrEmptyItems),
			errors.Is(err, order.ErrInvalidQuantity),
			errors.Is(err, order.ErrProductNotFound),
			errors.Is(err, order.ErrInvalidCoupon):
			WriteError(w, http.StatusUnprocessableEntity, "validation_error", err.Error())
		default:
			LoggerFromContext(r.Context(), nil).Error("failed to place order", "error", err)
			WriteError(w, http.StatusInternalServerError, "internal", "failed to place order")
		}
		return
	}

	WriteJSON(w, http.StatusOK, toOrderResponse(created))
}
