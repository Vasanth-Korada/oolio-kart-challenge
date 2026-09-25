package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/product"
)

// ProductHandler serves GET /product and GET /product/{id}.
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

func toProductResponse(item product.Product) productResponse {
	return productResponse{
		ID:       item.ID,
		Name:     item.Name,
		Price:    item.Price,
		Category: item.Category,
		Image: imageResponse{
			Thumbnail: item.Image.Thumbnail,
			Mobile:    item.Image.Mobile,
			Tablet:    item.Image.Tablet,
			Desktop:   item.Image.Desktop,
		},
	}
}

// List serves GET /product: every product in the catalog.
func (h *ProductHandler) List(w http.ResponseWriter, r *http.Request) {
	products, err := h.Service.List(r.Context())
	if err != nil {
		LoggerFromContext(r.Context(), nil).Error("failed to list products", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "failed to list products")
		return
	}

	resp := make([]productResponse, len(products))
	for index, item := range products {
		resp[index] = toProductResponse(item)
	}
	WriteJSON(w, http.StatusOK, resp)
}

// Get serves GET /product/{id}. The id must be an integer (400 otherwise) and
// is normalised, so "01" finds product "1".
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
