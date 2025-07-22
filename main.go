package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/shopspring/decimal"
)

// L3 Order Queue Structure
type OrderQueue struct {
	orders []decimal.Decimal // Individual orders in FIFO sequence
	mu     sync.RWMutex
}

func (oq *OrderQueue) sum() decimal.Decimal {
	total := decimal.Zero
	for _, order := range oq.orders {
		total = total.Add(order)
	}
	return total
}


func (oq *OrderQueue) largestOrderIndex() int {
	if len(oq.orders) == 0 {
		return -1
	}
	maxIdx := 0
	maxOrder := oq.orders[0]
	for i := 1; i < len(oq.orders); i++ {
		if oq.orders[i].GreaterThan(maxOrder) {
			maxOrder = oq.orders[i]
			maxIdx = i
		}
	}
	return maxIdx
}

// L3 Order Book Engine
type L3OrderBook struct {
	bids            map[string]*OrderQueue        // price -> order queue (legacy)
	asks            map[string]*OrderQueue        // price -> order queue (legacy)
	enhancedBids    map[string]*EnhancedOrderQueue // price -> enhanced order queue
	enhancedAsks    map[string]*EnhancedOrderQueue // price -> enhanced order queue
	symbol          string
	lastID          int64
	mu              sync.RWMutex
	kmeansMode      bool           // Whether to enable K-means clustering
	numClusters     int            // Number of clusters for K-means
	precision       *PrecisionInfo // Symbol precision information
	useEnhancedMode bool           // Whether to use enhanced queue management
	lastOptimization int64         // Last queue optimization timestamp
}

func NewL3OrderBook(symbol string) *L3OrderBook {
	// Initialize precision manager if not already done
	if precisionManager == nil {
		InitializePrecisionManager()
	}
	
	return &L3OrderBook{
		bids:             make(map[string]*OrderQueue),
		asks:             make(map[string]*OrderQueue),
		enhancedBids:     make(map[string]*EnhancedOrderQueue),
		enhancedAsks:     make(map[string]*EnhancedOrderQueue),
		symbol:           symbol,
		kmeansMode:       false, // Default to disabled
		numClusters:      10,    // Default number of clusters
		precision:        precisionManager.GetPrecisionInfo(symbol),
		useEnhancedMode:  true,  // Enable enhanced mode by default
		lastOptimization: time.Now().UnixMilli(),
	}
}

// Apply L2 snapshot to initialize L3 queues
func (ob *L3OrderBook) loadSnapshot(resp *binanceRESTResp) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	// Clear existing queues
	ob.bids = make(map[string]*OrderQueue)
	ob.asks = make(map[string]*OrderQueue)
	ob.enhancedBids = make(map[string]*EnhancedOrderQueue)
	ob.enhancedAsks = make(map[string]*EnhancedOrderQueue)

	// Initialize bid queues
	for _, bid := range resp.Bids {
		if len(bid) < 2 {
			continue
		}
		price := bid[0]
		qty, err := decimal.NewFromString(bid[1])
		if err != nil || qty.IsZero() {
			continue
		}

		// Legacy queue
		ob.bids[price] = &OrderQueue{
			orders: []decimal.Decimal{qty}, // Start with single order
		}

		// Enhanced queue
		if ob.useEnhancedMode {
			enhancedQueue := NewEnhancedOrderQueue(price)
			enhancedQueue.AddOrder(qty)
			ob.enhancedBids[price] = enhancedQueue
		}
	}

	// Initialize ask queues
	for _, ask := range resp.Asks {
		if len(ask) < 2 {
			continue
		}
		price := ask[0]
		qty, err := decimal.NewFromString(ask[1])
		if err != nil || qty.IsZero() {
			continue
		}

		// Legacy queue
		ob.asks[price] = &OrderQueue{
			orders: []decimal.Decimal{qty}, // Start with single order
		}

		// Enhanced queue
		if ob.useEnhancedMode {
			enhancedQueue := NewEnhancedOrderQueue(price)
			enhancedQueue.AddOrder(qty)
			ob.enhancedAsks[price] = enhancedQueue
		}
	}

	ob.lastID = resp.LastUpdateID
	log.Printf("L3 Order Book initialized with %d bid levels, %d ask levels",
		len(ob.bids), len(ob.asks))
}

// Apply L2 delta update to reconstruct L3 queues
func (ob *L3OrderBook) applyDelta(update *binanceWSUpdate) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	// Process bid updates
	for _, bid := range update.B {
		if len(bid) < 2 {
			continue
		}
		price := bid[0]
		newQty, err := decimal.NewFromString(bid[1])
		if err != nil {
			continue
		}

		if newQty.IsZero() {
			// Remove entire price level
			delete(ob.bids, price)
			if ob.useEnhancedMode {
				delete(ob.enhancedBids, price)
			}
		} else {
			ob.updateQueue(ob.bids, price, newQty)
			if ob.useEnhancedMode {
				ob.updateEnhancedQueue(ob.enhancedBids, price, newQty)
			}
		}
	}

	// Process ask updates
	for _, ask := range update.A {
		if len(ask) < 2 {
			continue
		}
		price := ask[0]
		newQty, err := decimal.NewFromString(ask[1])
		if err != nil {
			continue
		}

		if newQty.IsZero() {
			// Remove entire price level
			delete(ob.asks, price)
			if ob.useEnhancedMode {
				delete(ob.enhancedAsks, price)
			}
		} else {
			ob.updateQueue(ob.asks, price, newQty)
			if ob.useEnhancedMode {
				ob.updateEnhancedQueue(ob.enhancedAsks, price, newQty)
			}
		}
	}
}

// Core L3 Queue Reconstruction Algorithm (based on Rust implementation)
func (ob *L3OrderBook) updateQueue(side map[string]*OrderQueue, price string, newQty decimal.Decimal) {
	queue, exists := side[price]

	if !exists {
		// New price level - create initial queue
		side[price] = &OrderQueue{
			orders: []decimal.Decimal{newQty},
		}
		return
	}

	queue.mu.Lock()
	defer queue.mu.Unlock()

	oldSum := queue.sum()

	if newQty.GreaterThan(oldSum) {
		// Quantity increased - new order added to back of queue (FIFO)
		diff := newQty.Sub(oldSum)
		queue.orders = append(queue.orders, diff)

	} else if newQty.LessThan(oldSum) {
		// Quantity decreased - remove from largest order first
		diff := oldSum.Sub(newQty)

		// Find exact match for cancellation (Rust logic)
		removed := false
		for i := len(queue.orders) - 1; i >= 0; i-- {
			if queue.orders[i].Equal(diff) {
				// Remove exact matching order
				queue.orders = append(queue.orders[:i], queue.orders[i+1:]...)
				removed = true
				break
			}
		}

		if !removed {
			// No exact match - reduce largest order
			largestIdx := queue.largestOrderIndex()
			if largestIdx >= 0 {
				if queue.orders[largestIdx].GreaterThan(diff) {
					// Partial reduction of largest order
					queue.orders[largestIdx] = queue.orders[largestIdx].Sub(diff)
				} else {
					// Remove entire largest order
					queue.orders = append(queue.orders[:largestIdx], queue.orders[largestIdx+1:]...)
				}
			}
		}
	}
	// If quantities are equal, no change needed
}

// updateEnhancedQueue updates enhanced queue with improved algorithms
func (ob *L3OrderBook) updateEnhancedQueue(side map[string]*EnhancedOrderQueue, price string, newQty decimal.Decimal) {
	queue, exists := side[price]

	if !exists {
		// New price level - create initial queue
		newQueue := NewEnhancedOrderQueue(price)
		newQueue.AddOrder(newQty)
		side[price] = newQueue
		return
	}

	oldSum := queue.GetTotalQty()

	if newQty.GreaterThan(oldSum) {
		// Quantity increased - new order added
		diff := newQty.Sub(oldSum)
		queue.AddOrder(diff)
	} else if newQty.LessThan(oldSum) {
		// Quantity decreased - remove using enhanced algorithm
		diff := oldSum.Sub(newQty)
		queue.RemoveQty(diff)
	}
	// If quantities are equal, no change needed

	// Periodic optimization
	if time.Now().UnixMilli()-ob.lastOptimization > 30000 { // Every 30 seconds
		ob.optimizeAllQueues()
	}
}

// optimizeAllQueues performs maintenance on all enhanced queues
func (ob *L3OrderBook) optimizeAllQueues() {
	// Update ages for all orders
	for _, queue := range ob.enhancedBids {
		queue.UpdateAge()
		queue.OptimizeQueue()
	}
	for _, queue := range ob.enhancedAsks {
		queue.UpdateAge()
		queue.OptimizeQueue()
	}
	
	ob.lastOptimization = time.Now().UnixMilli()
	log.Printf("Optimized %d bid queues and %d ask queues", len(ob.enhancedBids), len(ob.enhancedAsks))
}

// Enhanced L3 snapshot with queue details
type L3Level struct {
	Price           decimal.Decimal      `json:"price"`
	TotalSize       decimal.Decimal      `json:"total_size"`
	OrderCount      int                  `json:"order_count"`
	Orders          []decimal.Decimal    `json:"orders,omitempty"`          // Individual orders for top levels
	ClusteredOrders []*ClusteredOrder    `json:"clustered_orders,omitempty"` // Orders with cluster information
	MaxOrder        decimal.Decimal      `json:"max_order"`
	AvgOrder        decimal.Decimal      `json:"avg_order"`
	Colors          []string             `json:"colors,omitempty"`           // Color information for visualization
	QueueMetrics    *QueueMetrics        `json:"queue_metrics,omitempty"`    // Enhanced queue metrics
	OrderDetails    []*OrderInfo         `json:"order_details,omitempty"`    // Detailed order information
}

type L3Snapshot struct {
	Bids        []L3Level      `json:"bids"`
	Asks        []L3Level      `json:"asks"`
	Timestamp   int64          `json:"timestamp"`
	Symbol      string         `json:"symbol"`
	KmeansMode  bool           `json:"kmeans_mode"`  // Whether clustering is enabled
	NumClusters int            `json:"num_clusters"` // Number of clusters used
	Precision   *PrecisionInfo `json:"precision"`    // Symbol precision information
}

func (ob *L3OrderBook) getL3Snapshot(topLevels int) L3Snapshot {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	// Get sorted bid prices (high to low)
	bidPrices := make([]string, 0, len(ob.bids))
	for price := range ob.bids {
		bidPrices = append(bidPrices, price)
	}
	sort.Slice(bidPrices, func(i, j int) bool {
		pi, _ := decimal.NewFromString(bidPrices[i])
		pj, _ := decimal.NewFromString(bidPrices[j])
		return pi.GreaterThan(pj)
	})

	// Get sorted ask prices (low to high)
	askPrices := make([]string, 0, len(ob.asks))
	for price := range ob.asks {
		askPrices = append(askPrices, price)
	}
	sort.Slice(askPrices, func(i, j int) bool {
		pi, _ := decimal.NewFromString(askPrices[i])
		pj, _ := decimal.NewFromString(askPrices[j])
		return pi.LessThan(pj)
	})

	// Perform clustering if enabled
	var clusteredBids, clusteredAsks map[string][]*ClusteredOrder
	if ob.kmeansMode {
		clusteredBids = ClusterOrderBook(ob.bids, ob.numClusters)
		clusteredAsks = ClusterOrderBook(ob.asks, ob.numClusters)
	}

	// Calculate max orders for special highlighting across all levels
	maxBidOrder := decimal.Zero
	secondMaxBidOrder := decimal.Zero
	maxAskOrder := decimal.Zero
	secondMaxAskOrder := decimal.Zero

	// Find max bid orders
	var allBidOrders []decimal.Decimal
	for _, queue := range ob.bids {
		queue.mu.RLock()
		for _, order := range queue.orders {
			if order.GreaterThan(decimal.Zero) {
				allBidOrders = append(allBidOrders, order)
			}
		}
		queue.mu.RUnlock()
	}
	if len(allBidOrders) > 0 {
		// Sort to find max and second max
		for _, order := range allBidOrders {
			if order.GreaterThan(maxBidOrder) {
				secondMaxBidOrder = maxBidOrder
				maxBidOrder = order
			} else if order.GreaterThan(secondMaxBidOrder) && !order.Equal(maxBidOrder) {
				secondMaxBidOrder = order
			}
		}
	}

	// Find max ask orders
	var allAskOrders []decimal.Decimal
	for _, queue := range ob.asks {
		queue.mu.RLock()
		for _, order := range queue.orders {
			if order.GreaterThan(decimal.Zero) {
				allAskOrders = append(allAskOrders, order)
			}
		}
		queue.mu.RUnlock()
	}
	if len(allAskOrders) > 0 {
		// Sort to find max and second max
		for _, order := range allAskOrders {
			if order.GreaterThan(maxAskOrder) {
				secondMaxAskOrder = maxAskOrder
				maxAskOrder = order
			} else if order.GreaterThan(secondMaxAskOrder) && !order.Equal(maxAskOrder) {
				secondMaxAskOrder = order
			}
		}
	}

	// Build L3 bid levels
	bids := make([]L3Level, 0, min(topLevels, len(bidPrices)))
	for i := 0; i < min(topLevels, len(bidPrices)); i++ {
		price := bidPrices[i]
		queue := ob.bids[price]
		queue.mu.RLock()

		priceDecimal, _ := decimal.NewFromString(price)
		totalSize := queue.sum()
		orderCount := len(queue.orders)

		var maxOrder, avgOrder decimal.Decimal
		if orderCount > 0 {
			maxOrder = queue.orders[0]
			for _, order := range queue.orders {
				if order.GreaterThan(maxOrder) {
					maxOrder = order
				}
			}
			avgOrder = totalSize.Div(decimal.NewFromInt(int64(orderCount)))
		}

		level := L3Level{
			Price:      priceDecimal,
			TotalSize:  totalSize,
			OrderCount: orderCount,
			MaxOrder:   maxOrder,
			AvgOrder:   avgOrder,
		}

		// Include individual orders and clustering for all visible levels
		if i < topLevels {
			level.Orders = make([]decimal.Decimal, len(queue.orders))
			copy(level.Orders, queue.orders)

			// Include enhanced queue information if available
			if ob.useEnhancedMode {
				if enhancedQueue, exists := ob.enhancedBids[price]; exists {
					metrics := enhancedQueue.GetMetrics()
					level.QueueMetrics = &metrics
					level.OrderDetails = enhancedQueue.GetOrders()
				}
			}

			// Generate colors based on mode
			if ob.kmeansMode {
				// Add clustered orders if clustering is enabled
				if clusteredOrders, exists := clusteredBids[price]; exists {
					level.ClusteredOrders = clusteredOrders
					level.Colors = GenerateClusteredOrderColors(clusteredOrders, true, maxBidOrder, secondMaxBidOrder)
				}
			} else {
				// Generate age-based colors for normal mode
				level.Colors = GenerateOrderColors(queue.orders, true, maxBidOrder, secondMaxBidOrder)
			}
		}

		bids = append(bids, level)
		queue.mu.RUnlock()
	}

	// Build L3 ask levels
	asks := make([]L3Level, 0, min(topLevels, len(askPrices)))
	for i := 0; i < min(topLevels, len(askPrices)); i++ {
		price := askPrices[i]
		queue := ob.asks[price]
		queue.mu.RLock()

		priceDecimal, _ := decimal.NewFromString(price)
		totalSize := queue.sum()
		orderCount := len(queue.orders)

		var maxOrder, avgOrder decimal.Decimal
		if orderCount > 0 {
			maxOrder = queue.orders[0]
			for _, order := range queue.orders {
				if order.GreaterThan(maxOrder) {
					maxOrder = order
				}
			}
			avgOrder = totalSize.Div(decimal.NewFromInt(int64(orderCount)))
		}

		level := L3Level{
			Price:      priceDecimal,
			TotalSize:  totalSize,
			OrderCount: orderCount,
			MaxOrder:   maxOrder,
			AvgOrder:   avgOrder,
		}

		// Include individual orders and clustering for all visible levels
		if i < topLevels {
			level.Orders = make([]decimal.Decimal, len(queue.orders))
			copy(level.Orders, queue.orders)

			// Include enhanced queue information if available
			if ob.useEnhancedMode {
				if enhancedQueue, exists := ob.enhancedAsks[price]; exists {
					metrics := enhancedQueue.GetMetrics()
					level.QueueMetrics = &metrics
					level.OrderDetails = enhancedQueue.GetOrders()
				}
			}

			// Generate colors based on mode
			if ob.kmeansMode {
				// Add clustered orders if clustering is enabled
				if clusteredOrders, exists := clusteredAsks[price]; exists {
					level.ClusteredOrders = clusteredOrders
					level.Colors = GenerateClusteredOrderColors(clusteredOrders, false, maxAskOrder, secondMaxAskOrder)
				}
			} else {
				// Generate age-based colors for normal mode
				level.Colors = GenerateOrderColors(queue.orders, false, maxAskOrder, secondMaxAskOrder)
			}
		}

		asks = append(asks, level)
		queue.mu.RUnlock()
	}

	return L3Snapshot{
		Bids:        bids,
		Asks:        asks,
		Timestamp:   time.Now().UnixMilli(),
		Symbol:      ob.symbol,
		KmeansMode:  ob.kmeansMode,
		NumClusters: ob.numClusters,
		Precision:   ob.precision,
	}
}

// SetKmeansMode enables or disables K-means clustering
func (ob *L3OrderBook) SetKmeansMode(enabled bool) {
	ob.mu.Lock()
	defer ob.mu.Unlock()
	ob.kmeansMode = enabled
}

// SetNumClusters sets the number of clusters for K-means
func (ob *L3OrderBook) SetNumClusters(clusters int) {
	ob.mu.Lock()
	defer ob.mu.Unlock()
	if clusters > 0 && clusters <= 20 { // Reasonable limits
		ob.numClusters = clusters
	}
}

// GetClusteringInfo returns current clustering configuration
func (ob *L3OrderBook) GetClusteringInfo() (bool, int) {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	return ob.kmeansMode, ob.numClusters
}

// RefreshPrecision refreshes precision information for the symbol
func (ob *L3OrderBook) RefreshPrecision() {
	ob.mu.Lock()
	defer ob.mu.Unlock()
	if precisionManager != nil {
		ob.precision = precisionManager.GetPrecisionInfo(ob.symbol)
	}
}

// Rest of the implementation (WebSocket, HTTP handlers) remains the same
type binanceWSUpdate struct {
	U int64      `json:"U"`
	u int64      `json:"u"`
	B [][]string `json:"b"`
	A [][]string `json:"a"`
}

type binanceRESTResp struct {
	LastUpdateID int64      `json:"lastUpdateId"`
	Bids         [][]string `json:"bids"`
	Asks         [][]string `json:"asks"`
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Global state for symbol switching
type AppState struct {
	book         *L3OrderBook
	currentSymbol string
	binanceCancel chan bool
	mu           sync.RWMutex
}

var appState *AppState

func main() {
	symbol := "ethusdt" // Default to ETHUSDT
	if len(os.Args) > 1 {
		symbol = strings.ToLower(os.Args[1])
	}

	appState = &AppState{
		book:          NewL3OrderBook(symbol),
		currentSymbol: symbol,
		binanceCancel: make(chan bool, 1),
	}

	go runBinanceSync(symbol, appState.book, appState.binanceCancel)

	http.Handle("/", http.FileServer(http.Dir("static")))
	http.HandleFunc("/ws", wsHandler())

	log.Printf("L3 Order Book Server running on http://localhost:8080")
	log.Printf("Symbol: %s", strings.ToUpper(symbol))
	log.Fatal(http.ListenAndServe(":8080", nil))
}

type WSMessage struct {
	Type        string `json:"type"`
	Symbol      string `json:"symbol,omitempty"`
	KmeansMode  *bool  `json:"kmeans_mode,omitempty"`
	NumClusters *int   `json:"num_clusters,omitempty"`
}

func wsHandler() http.HandlerFunc {
	var upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("upgrade err:", err)
			return
		}
		defer conn.Close()

		ticker := time.NewTicker(50 * time.Millisecond) // 20 FPS for L3 data
		defer ticker.Stop()

		// Handle incoming messages for symbol switching
		go func() {
			for {
				var msg WSMessage
				if err := conn.ReadJSON(&msg); err != nil {
					log.Println("WebSocket read error:", err)
					return
				}

				switch msg.Type {
				case "switch_symbol":
					if msg.Symbol != "" {
						newSymbol := strings.ToLower(msg.Symbol)
						log.Printf("Switching to symbol: %s", strings.ToUpper(newSymbol))
						
						// Switch symbol
						if err := switchSymbol(newSymbol); err != nil {
							errorMsg := map[string]any{
								"type":    "error",
								"message": err.Error(),
							}
							conn.WriteJSON(errorMsg)
						} else {
							// Notify successful switch
							switchMsg := map[string]any{
								"type":   "symbol_switched",
								"symbol": strings.ToUpper(newSymbol),
							}
							conn.WriteJSON(switchMsg)
						}
					}

				case "toggle_kmeans":
					appState.mu.Lock()
					if msg.KmeansMode != nil {
						appState.book.SetKmeansMode(*msg.KmeansMode)
						log.Printf("K-means mode set to: %t", *msg.KmeansMode)
					}
					if msg.NumClusters != nil {
						appState.book.SetNumClusters(*msg.NumClusters)
						log.Printf("Number of clusters set to: %d", *msg.NumClusters)
					}
					
					enabled, clusters := appState.book.GetClusteringInfo()
					appState.mu.Unlock()
					
					// Send confirmation
					responseMsg := map[string]any{
						"type":         "kmeans_updated",
						"kmeans_mode":  enabled,
						"num_clusters": clusters,
					}
					conn.WriteJSON(responseMsg)

				case "get_clustering_info":
					appState.mu.RLock()
					enabled, clusters := appState.book.GetClusteringInfo()
					appState.mu.RUnlock()
					
					responseMsg := map[string]any{
						"type":         "clustering_info",
						"kmeans_mode":  enabled,
						"num_clusters": clusters,
					}
					conn.WriteJSON(responseMsg)

				case "refresh_precision":
					appState.mu.Lock()
					appState.book.RefreshPrecision()
					appState.mu.Unlock()
					
					responseMsg := map[string]any{
						"type":    "precision_refreshed",
						"message": "Precision information updated",
					}
					conn.WriteJSON(responseMsg)

				case "get_precision_info":
					appState.mu.RLock()
					precision := appState.book.precision
					appState.mu.RUnlock()
					
					responseMsg := map[string]any{
						"type":      "precision_info",
						"precision": precision,
					}
					conn.WriteJSON(responseMsg)
				}
			}
		}()

		for {
			select {
			case <-ticker.C:
				appState.mu.RLock()
				snapshot := appState.book.getL3Snapshot(100)
				appState.mu.RUnlock()
				
				message := map[string]any{
					"type": "l3_update",
					"data": snapshot,
				}

				if err := conn.WriteJSON(message); err != nil {
					return
				}
			}
		}
	}
}

func switchSymbol(newSymbol string) error {
	appState.mu.Lock()
	defer appState.mu.Unlock()

	if appState.currentSymbol == newSymbol {
		return nil // Already on this symbol
	}

	// Cancel current Binance connection
	select {
	case appState.binanceCancel <- true:
	default:
	}

	// Create new book and start new connection
	appState.book = NewL3OrderBook(newSymbol)
	appState.currentSymbol = newSymbol
	appState.binanceCancel = make(chan bool, 1)

	go runBinanceSync(newSymbol, appState.book, appState.binanceCancel)

	return nil
}

func runBinanceSync(symbol string, book *L3OrderBook, cancel chan bool) {
	for {
		select {
		case <-cancel:
			log.Printf("Cancelling Binance sync for %s", strings.ToUpper(symbol))
			return
		default:
			if err := connectAndSync(symbol, book, cancel); err != nil {
				log.Printf("Connection failed for %s: %v, retrying in 5s...", strings.ToUpper(symbol), err)
				time.Sleep(5 * time.Second)
				continue
			}
		}
	}
}

func connectAndSync(symbol string, book *L3OrderBook, cancel chan bool) error {
	wsURL := fmt.Sprintf("wss://fstream.binance.com/ws/%s@depth@100ms", symbol)

	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("cannot dial Binance WS: %w", err)
	}
	defer ws.Close()

	log.Println("Connected Binance WS:", wsURL)

	// Fetch initial snapshot
	snapURL := fmt.Sprintf("https://fapi.binance.com/fapi/v1/depth?symbol=%s&limit=1000",
		strings.ToUpper(symbol))

	var snapResp binanceRESTResp
	for {
		select {
		case <-cancel:
			return fmt.Errorf("cancelled during snapshot fetch")
		default:
			resp, err := http.Get(snapURL)
			if err == nil && resp.StatusCode == 200 {
				err2 := json.NewDecoder(resp.Body).Decode(&snapResp)
				resp.Body.Close()
				if err2 == nil && snapResp.LastUpdateID != 0 {
					goto snapshotLoaded
				}
			}
			if resp != nil {
				resp.Body.Close()
			}
			time.Sleep(200 * time.Millisecond)
		}
	}

snapshotLoaded:
	book.loadSnapshot(&snapResp)
	log.Printf("L3 Order Book snapshot loaded: %d", snapResp.LastUpdateID)

	// Process real-time updates
	for {
		select {
		case <-cancel:
			log.Printf("Cancelling Binance sync for %s", strings.ToUpper(symbol))
			return fmt.Errorf("cancelled")
		default:
			// Set a reasonable read deadline
			ws.SetReadDeadline(time.Now().Add(1 * time.Second))
			_, msg, err := ws.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					return fmt.Errorf("websocket read error: %w", err)
				}
				// Handle timeout or normal close
				if netErr, ok := err.(interface{ Timeout() bool }); ok && netErr.Timeout() {
					continue // Timeout, check cancel channel again
				}
				return fmt.Errorf("websocket error: %w", err)
			}

			var update binanceWSUpdate
			if err := json.Unmarshal(msg, &update); err != nil {
				log.Printf("Failed to unmarshal update: %v", err)
				continue
			}

			book.applyDelta(&update)
		}
	}
}
