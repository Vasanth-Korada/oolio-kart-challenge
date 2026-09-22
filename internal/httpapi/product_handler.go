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

type imageResponse struct {
	Thumbnail string `json:"thumbnail"`
	Mobile    string `json:"mobile"`
	Tablet    string `json:"tablet"`
	Desktop   string `json:"desktop"`
}

type productResponse struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	Price    float64       `json:"price"`
	Category string        `json:"category"`
	Image    imageResponse `json:"image"`
}

func toProductResponse(p product.Product) productResponse {
	return productResponse{
		ID:       p.ID,
		Name:     p.Name,
		Price:    p.Price,
		Category: p.Category,
		Image: imageResponse{
			Thumbnail: p.Image.Thumbnail,
			Mobile:    p.Image.Mobile,
			Tablet:    p.Image.Tablet,
			Desktop:   p.Image.Desktop,
		},
	}
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
// The parsed-and-reformatted value (not the raw path segment) is what
// gets looked up, so "01" and "1" resolve to the same product instead
// of "01" passing validation but missing the lookup.
func (h *ProductHandler) Get(w http.ResponseWriter, r *http.Request) {
	idParam := r.PathValue("id")
	parsedID, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "product id must be an integer")
		return
	}
	canonicalID := strconv.FormatInt(parsedID, 10)

	p, err := h.Service.Get(r.Context(), canonicalID)
	if err != nil {
		if errors.Is(err, product.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "not_found", "product not found")
			return
		}
		LoggerFromContext(r.Context(), nil).Error("failed to get product", "error", err, "product_id", canonicalID)
		WriteError(w, http.StatusInternalServerError, "internal", "failed to fetch product")
		return
	}
	WriteJSON(w, http.StatusOK, toProductResponse(p))
}
