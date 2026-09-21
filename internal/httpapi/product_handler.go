package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/product"
)

type ProductHandler struct {
	Service product.Service
}

type productResponse struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	Category string  `json:"category"`
}

func toProductResponse(p product.Product) productResponse {
	return productResponse{ID: p.ID, Name: p.Name, Price: p.Price, Category: p.Category}
}

func (h *ProductHandler) List(w http.ResponseWriter, r *http.Request) {
	products, err := h.Service.List(r.Context())
	if err != nil {
		LoggerFromContext(r.Context(), nil).Error("failed to list products", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "failed to list products")
		return
	}

	resp := make([]productResponse, len(products))
	for i, p := range products {
		resp[i] = toProductResponse(p)
	}
	WriteJSON(w, http.StatusOK, resp)
}

// productId is spec'd as an integer, so a non-numeric id is a 400.
func (h *ProductHandler) Get(w http.ResponseWriter, r *http.Request) {
	idParam := r.PathValue("id")
	if _, err := strconv.ParseInt(idParam, 10, 64); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "product id must be an integer")
		return
	}

	p, err := h.Service.Get(r.Context(), idParam)
	if err != nil {
		if errors.Is(err, product.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "not_found", "product not found")
			return
		}
		LoggerFromContext(r.Context(), nil).Error("failed to get product", "error", err, "product_id", idParam)
		WriteError(w, http.StatusInternalServerError, "internal", "failed to fetch product")
		return
	}
	WriteJSON(w, http.StatusOK, toProductResponse(p))
}
