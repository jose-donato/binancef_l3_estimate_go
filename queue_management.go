package main

import (
	"sort"
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

// OrderInfo represents detailed order information for better tracking
type OrderInfo struct {
	ID        uint64          `json:"id"`         // Synthetic order ID
	Qty       decimal.Decimal `json:"qty"`        // Order quantity
	Timestamp int64           `json:"timestamp"`  // Creation timestamp
	Age       int64           `json:"age"`        // Age in milliseconds
	IsPartial bool            `json:"is_partial"` // Whether this order was partially filled
}

// EnhancedOrderQueue provides advanced order queue management
type EnhancedOrderQueue struct {
	orders      []*OrderInfo    // FIFO ordered list of orders
	totalQty    decimal.Decimal // Cache for total quantity
	nextOrderID uint64          // Counter for synthetic order IDs
	mu          sync.RWMutex
	priceLevel  string          // Price level this queue represents
	lastUpdate  int64           // Last update timestamp
}

// NewEnhancedOrderQueue creates a new enhanced order queue
func NewEnhancedOrderQueue(priceLevel string) *EnhancedOrderQueue {
	return &EnhancedOrderQueue{
		orders:      make([]*OrderInfo, 0),
		totalQty:    decimal.Zero,
		nextOrderID: 1,
		priceLevel:  priceLevel,
		lastUpdate:  time.Now().UnixMilli(),
	}
}

// AddOrder adds a new order to the queue
func (eq *EnhancedOrderQueue) AddOrder(qty decimal.Decimal) {
	eq.mu.Lock()
	defer eq.mu.Unlock()

	now := time.Now().UnixMilli()
	order := &OrderInfo{
		ID:        eq.nextOrderID,
		Qty:       qty,
		Timestamp: now,
		Age:       0,
		IsPartial: false,
	}
	
	eq.nextOrderID++
	eq.orders = append(eq.orders, order)
	eq.totalQty = eq.totalQty.Add(qty)
	eq.lastUpdate = now
}

// RemoveQty removes quantity from the queue using FIFO and intelligent matching
func (eq *EnhancedOrderQueue) RemoveQty(qtyToRemove decimal.Decimal) {
	eq.mu.Lock()
	defer eq.mu.Unlock()

	if qtyToRemove.LessThanOrEqual(decimal.Zero) {
		return
	}

	remaining := qtyToRemove
	now := time.Now().UnixMilli()

	// Strategy 1: Try to find exact match first (simulates order cancellation)
	for i := len(eq.orders) - 1; i >= 0; i-- {
		if eq.orders[i].Qty.Equal(remaining) {
			// Exact match - remove entire order
			eq.totalQty = eq.totalQty.Sub(eq.orders[i].Qty)
			eq.orders = append(eq.orders[:i], eq.orders[i+1:]...)
			eq.lastUpdate = now
			return
		}
	}

	// Strategy 2: Remove from largest orders first (simulates large order fills)
	if remaining.GreaterThan(eq.getLargestOrderQty().Div(decimal.NewFromFloat(2))) {
		eq.removeFromLargestOrders(&remaining)
	} else {
		// Strategy 3: FIFO removal for small changes (simulates normal fills)
		eq.removeFIFO(&remaining)
	}

	eq.lastUpdate = now
}

// removeFIFO removes quantity using FIFO order (front of queue first)
func (eq *EnhancedOrderQueue) removeFIFO(remaining *decimal.Decimal) {
	i := 0
	for i < len(eq.orders) && remaining.GreaterThan(decimal.Zero) {
		order := eq.orders[i]
		
		if order.Qty.LessThanOrEqual(*remaining) {
			// Remove entire order
			*remaining = remaining.Sub(order.Qty)
			eq.totalQty = eq.totalQty.Sub(order.Qty)
			eq.orders = append(eq.orders[:i], eq.orders[i+1:]...)
			// Don't increment i since we removed an element
		} else {
			// Partial fill
			fillAmount := *remaining
			order.Qty = order.Qty.Sub(fillAmount)
			order.IsPartial = true
			eq.totalQty = eq.totalQty.Sub(fillAmount)
			*remaining = decimal.Zero
			i++
		}
	}
}

// removeFromLargestOrders removes quantity from the largest orders first
func (eq *EnhancedOrderQueue) removeFromLargestOrders(remaining *decimal.Decimal) {
	for remaining.GreaterThan(decimal.Zero) && len(eq.orders) > 0 {
		// Find largest order
		largestIdx := eq.getLargestOrderIndex()
		if largestIdx == -1 {
			break
		}

		largestOrder := eq.orders[largestIdx]
		
		if largestOrder.Qty.LessThanOrEqual(*remaining) {
			// Remove entire largest order
			*remaining = remaining.Sub(largestOrder.Qty)
			eq.totalQty = eq.totalQty.Sub(largestOrder.Qty)
			eq.orders = append(eq.orders[:largestIdx], eq.orders[largestIdx+1:]...)
		} else {
			// Partial fill of largest order
			fillAmount := *remaining
			largestOrder.Qty = largestOrder.Qty.Sub(fillAmount)
			largestOrder.IsPartial = true
			eq.totalQty = eq.totalQty.Sub(fillAmount)
			*remaining = decimal.Zero
		}
	}
}

// getLargestOrderIndex finds the index of the largest order
func (eq *EnhancedOrderQueue) getLargestOrderIndex() int {
	if len(eq.orders) == 0 {
		return -1
	}

	maxIdx := 0
	maxQty := eq.orders[0].Qty
	
	for i := 1; i < len(eq.orders); i++ {
		if eq.orders[i].Qty.GreaterThan(maxQty) {
			maxQty = eq.orders[i].Qty
			maxIdx = i
		}
	}
	
	return maxIdx
}

// getLargestOrderQty returns the quantity of the largest order
func (eq *EnhancedOrderQueue) getLargestOrderQty() decimal.Decimal {
	idx := eq.getLargestOrderIndex()
	if idx == -1 {
		return decimal.Zero
	}
	return eq.orders[idx].Qty
}

// UpdateAge updates the age of all orders in the queue
func (eq *EnhancedOrderQueue) UpdateAge() {
	eq.mu.Lock()
	defer eq.mu.Unlock()

	now := time.Now().UnixMilli()
	for _, order := range eq.orders {
		order.Age = now - order.Timestamp
	}
}

// GetOrders returns a copy of all orders (thread-safe)
func (eq *EnhancedOrderQueue) GetOrders() []*OrderInfo {
	eq.mu.RLock()
	defer eq.mu.RUnlock()

	orders := make([]*OrderInfo, len(eq.orders))
	for i, order := range eq.orders {
		orders[i] = &OrderInfo{
			ID:        order.ID,
			Qty:       order.Qty,
			Timestamp: order.Timestamp,
			Age:       order.Age,
			IsPartial: order.IsPartial,
		}
	}
	return orders
}

// GetTotalQty returns the total quantity in the queue
func (eq *EnhancedOrderQueue) GetTotalQty() decimal.Decimal {
	eq.mu.RLock()
	defer eq.mu.RUnlock()
	return eq.totalQty
}

// GetOrderCount returns the number of orders in the queue
func (eq *EnhancedOrderQueue) GetOrderCount() int {
	eq.mu.RLock()
	defer eq.mu.RUnlock()
	return len(eq.orders)
}

// GetAverageOrderAge returns the average age of orders in milliseconds
func (eq *EnhancedOrderQueue) GetAverageOrderAge() float64 {
	eq.mu.RLock()
	defer eq.mu.RUnlock()

	if len(eq.orders) == 0 {
		return 0
	}

	totalAge := int64(0)
	now := time.Now().UnixMilli()
	
	for _, order := range eq.orders {
		age := now - order.Timestamp
		totalAge += age
	}

	return float64(totalAge) / float64(len(eq.orders))
}

// GetQueueDepthMetrics returns detailed metrics about the queue
type QueueMetrics struct {
	TotalOrders    int             `json:"total_orders"`
	TotalQty       decimal.Decimal `json:"total_qty"`
	AvgOrderSize   decimal.Decimal `json:"avg_order_size"`
	MaxOrderSize   decimal.Decimal `json:"max_order_size"`
	MinOrderSize   decimal.Decimal `json:"min_order_size"`
	AvgAge         float64         `json:"avg_age_ms"`
	OldestAge      int64           `json:"oldest_age_ms"`
	PartialOrders  int             `json:"partial_orders"`
	LastUpdate     int64           `json:"last_update"`
}

// GetMetrics returns comprehensive queue metrics
func (eq *EnhancedOrderQueue) GetMetrics() QueueMetrics {
	eq.mu.RLock()
	defer eq.mu.RUnlock()

	metrics := QueueMetrics{
		TotalOrders: len(eq.orders),
		TotalQty:    eq.totalQty,
		LastUpdate:  eq.lastUpdate,
	}

	if len(eq.orders) == 0 {
		return metrics
	}

	// Calculate min/max/average order sizes
	totalAge := int64(0)
	now := time.Now().UnixMilli()
	partialCount := 0
	
	minQty := eq.orders[0].Qty
	maxQty := eq.orders[0].Qty
	oldestAge := now - eq.orders[0].Timestamp

	for _, order := range eq.orders {
		age := now - order.Timestamp
		totalAge += age
		
		if age > oldestAge {
			oldestAge = age
		}
		
		if order.Qty.LessThan(minQty) {
			minQty = order.Qty
		}
		if order.Qty.GreaterThan(maxQty) {
			maxQty = order.Qty
		}
		
		if order.IsPartial {
			partialCount++
		}
	}

	metrics.AvgOrderSize = eq.totalQty.Div(decimal.NewFromInt(int64(len(eq.orders))))
	metrics.MaxOrderSize = maxQty
	metrics.MinOrderSize = minQty
	metrics.AvgAge = float64(totalAge) / float64(len(eq.orders))
	metrics.OldestAge = oldestAge
	metrics.PartialOrders = partialCount

	return metrics
}

// OptimizeQueue performs maintenance on the queue (merge small orders, clean up, etc.)
func (eq *EnhancedOrderQueue) OptimizeQueue() {
	eq.mu.Lock()
	defer eq.mu.Unlock()

	// Remove orders with zero quantity
	filteredOrders := make([]*OrderInfo, 0, len(eq.orders))
	for _, order := range eq.orders {
		if order.Qty.GreaterThan(decimal.Zero) {
			filteredOrders = append(filteredOrders, order)
		}
	}
	eq.orders = filteredOrders

	// Recalculate total quantity
	eq.totalQty = decimal.Zero
	for _, order := range eq.orders {
		eq.totalQty = eq.totalQty.Add(order.Qty)
	}

	// Sort orders by timestamp to maintain FIFO order
	sort.Slice(eq.orders, func(i, j int) bool {
		return eq.orders[i].Timestamp < eq.orders[j].Timestamp
	})

	eq.lastUpdate = time.Now().UnixMilli()
}

// Clear removes all orders from the queue
func (eq *EnhancedOrderQueue) Clear() {
	eq.mu.Lock()
	defer eq.mu.Unlock()

	eq.orders = eq.orders[:0]
	eq.totalQty = decimal.Zero
	eq.lastUpdate = time.Now().UnixMilli()
}

// GetOrdersByAge returns orders sorted by age (oldest first)
func (eq *EnhancedOrderQueue) GetOrdersByAge() []*OrderInfo {
	orders := eq.GetOrders()
	
	// Update ages
	now := time.Now().UnixMilli()
	for _, order := range orders {
		order.Age = now - order.Timestamp
	}
	
	// Sort by age (oldest first)
	sort.Slice(orders, func(i, j int) bool {
		return orders[i].Age > orders[j].Age
	})
	
	return orders
}