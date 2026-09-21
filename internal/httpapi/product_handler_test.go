package httpapi_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/httpapi"
	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/product"
)

type fakeProductService struct {
	products map[string]product.Product
	listErr  error
	getErr   error
}

func (f *fakeProductService) List(context.Context) ([]product.Product, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := make([]product.Product, 0, len(f.products))
	for _, p := range f.products {
		out = append(out, p)
	}
	return out, nil
}

func (f *fakeProductService) Get(_ context.Context, id string) (product.Product, error) {
	if f.getErr != nil {
		return product.Product{}, f.getErr
	}
	p, ok := f.products[id]
	if !ok {
		return product.Product{}, product.ErrNotFound
	}
	return p, nil
}

func TestProductHandler_List(t *testing.T) {
	tests := []struct {
		name       string
		service    *fakeProductService
		wantStatus int
	}{
		{
			name:       "success",
			service:    &fakeProductService{products: map[string]product.Product{"1": {ID: "1", Name: "Waffle"}}},
			wantStatus: http.StatusOK,
		},
		{
			name:       "service error",
			service:    &fakeProductService{listErr: errors.New("db down")},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &httpapi.ProductHandler{Service: tt.service}
			rec := httptest.NewRecorder()
			h.List(rec, httptest.NewRequest(http.MethodGet, "/product", nil))

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestProductHandler_Get(t *testing.T) {
	service := &fakeProductService{products: map[string]product.Product{"1": {ID: "1", Name: "Waffle"}}}
	h := &httpapi.ProductHandler{Service: service}

	tests := []struct {
		name       string
		id         string
		wantStatus int
	}{
		{name: "found", id: "1", wantStatus: http.StatusOK},
		{name: "not found", id: "999", wantStatus: http.StatusNotFound},
		{name: "non-numeric id", id: "abc", wantStatus: http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/product/"+tt.id, nil)
			req.SetPathValue("id", tt.id)
			rec := httptest.NewRecorder()
			h.Get(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}
