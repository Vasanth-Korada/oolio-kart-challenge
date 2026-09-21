package httpapi_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/httpapi"
	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/order"
)

type fakeOrderService struct {
	result order.Order
	err    error
}

func (f *fakeOrderService) PlaceOrder(context.Context, order.CreateOrderRequest) (order.Order, error) {
	return f.result, f.err
}

func TestOrderHandler_Create(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		service    *fakeOrderService
		wantStatus int
	}{
		{
			name:       "malformed json",
			body:       `{not json`,
			service:    &fakeOrderService{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty items rejected by service",
			body:       `{"items":[]}`,
			service:    &fakeOrderService{err: order.ErrEmptyItems},
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "invalid coupon rejected by service",
			body:       `{"items":[{"productId":"1","quantity":1}],"couponCode":"BAD"}`,
			service:    &fakeOrderService{err: order.ErrInvalidCoupon},
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "unexpected service error",
			body:       `{"items":[{"productId":"1","quantity":1}]}`,
			service:    &fakeOrderService{err: errors.New("db down")},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name: "success",
			body: `{"items":[{"productId":"1","quantity":1}]}`,
			service: &fakeOrderService{result: order.Order{
				ID:    "order-1",
				Items: []order.Item{{ProductID: "1", Quantity: 1}},
			}},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &httpapi.OrderHandler{Service: tt.service}
			req := httptest.NewRequest(http.MethodPost, "/order", bytes.NewBufferString(tt.body))
			rec := httptest.NewRecorder()
			h.Create(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body=%s", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}
