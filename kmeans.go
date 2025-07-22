package main

import (
	"math"
	"math/rand"
	"sort"
	"sync"

	"github.com/shopspring/decimal"
)

// Global K-means instances for persistent centroids
var (
	globalBidKMeans  *MiniBatchKMeans
	globalAskKMeans  *MiniBatchKMeans
	kmeansInitMutex  sync.Mutex
)

// Point structure for clustering (using qty only for simplicity)
type Point struct {
	qty float64
}

// MiniBatchKMeans implements the mini-batch K-means algorithm for order clustering
type MiniBatchKMeans struct {
	numClusters int
	batchSize   int
	maxIter     int
	centroids   []Point
	mu          sync.RWMutex
}

// NewMiniBatchKMeans creates a new MiniBatchKMeans instance
func NewMiniBatchKMeans(numClusters, batchSize, maxIter int) *MiniBatchKMeans {
	return &MiniBatchKMeans{
		numClusters: numClusters,
		batchSize:   batchSize,
		maxIter:     maxIter,
		centroids:   make([]Point, 0),
	}
}

// euclideanDistance calculates the Euclidean distance between two points
func euclideanDistance(a, b Point) float64 {
	return math.Abs(a.qty - b.qty)
}

// normalize normalizes the points to [0, 1] range
func normalize(points []Point) []Point {
	if len(points) == 0 {
		return points
	}

	minQty := math.MaxFloat64
	maxQty := -math.MaxFloat64

	for _, p := range points {
		if p.qty < minQty {
			minQty = p.qty
		}
		if p.qty > maxQty {
			maxQty = p.qty
		}
	}

	rangeQty := maxQty - minQty
	if rangeQty == 0 {
		return points // All points have the same quantity
	}

	normalized := make([]Point, len(points))
	for i, p := range points {
		normalized[i] = Point{qty: (p.qty - minQty) / rangeQty}
	}

	return normalized
}

// initializeCentroids initializes centroids using deterministic approach
func (kmeans *MiniBatchKMeans) initializeCentroids(points []Point) {
	if len(points) == 0 {
		return
	}

	// Sort points by quantity for deterministic initialization
	sorted := make([]Point, len(points))
	copy(sorted, points)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].qty < sorted[j].qty
	})

	// Pick evenly spaced points as initial centroids
	kmeans.centroids = make([]Point, kmeans.numClusters)
	step := len(sorted) / kmeans.numClusters
	if step == 0 {
		step = 1
	}

	for i := 0; i < kmeans.numClusters; i++ {
		idx := i * step
		if idx >= len(sorted) {
			idx = len(sorted) - 1
		}
		kmeans.centroids[i] = sorted[idx]
	}

	// Fill remaining centroids if needed
	for len(kmeans.centroids) < kmeans.numClusters {
		kmeans.centroids = append(kmeans.centroids, sorted[0])
	}
}

// closestCentroid finds the index of the closest centroid to a point
func (kmeans *MiniBatchKMeans) closestCentroid(p Point) int {
	minDist := math.Inf(1)
	minIdx := 0

	for i, c := range kmeans.centroids {
		dist := euclideanDistance(p, c)
		if dist < minDist {
			minDist = dist
			minIdx = i
		}
	}

	return minIdx
}

// Fit performs mini-batch K-means clustering on the order book data
func (kmeans *MiniBatchKMeans) Fit(orderBook map[string]*OrderQueue) []int {
	kmeans.mu.Lock()
	defer kmeans.mu.Unlock()

	var points []Point
	var orderList []struct {
		price string
		qty   decimal.Decimal
	}

	// Extract points from order book
	for price, queue := range orderBook {
		queue.mu.RLock()
		for _, qty := range queue.orders {
			if qty.GreaterThan(decimal.Zero) {
				qtyFloat, _ := qty.Float64()
				points = append(points, Point{qty: qtyFloat})
				orderList = append(orderList, struct {
					price string
					qty   decimal.Decimal
				}{price: price, qty: qty})
			}
		}
		queue.mu.RUnlock()
	}

	if len(points) == 0 {
		return []int{}
	}

	// Normalize points
	points = normalize(points)

	// Initialize centroids if not already set or if size changed
	if len(kmeans.centroids) == 0 || len(kmeans.centroids) != kmeans.numClusters {
		kmeans.initializeCentroids(points)
	}

	// Mini-batch updates with deterministic seed for stability
	rng := rand.New(rand.NewSource(42)) // Fixed seed for consistent results
	for iter := 0; iter < kmeans.maxIter; iter++ {
		// Select mini-batch
		batchSize := kmeans.batchSize
		if batchSize > len(points) {
			batchSize = len(points)
		}

		batchIndices := make([]int, batchSize)
		for i := 0; i < batchSize; i++ {
			batchIndices[i] = rng.Intn(len(points))
		}

		// Update centroids based on mini-batch
		counts := make([]int, kmeans.numClusters)
		sums := make([]float64, kmeans.numClusters)

		for _, idx := range batchIndices {
			p := points[idx]
			closest := kmeans.closestCentroid(p)
			sums[closest] += p.qty
			counts[closest]++
		}

		// Apply updates with learning rate
		for i := 0; i < kmeans.numClusters; i++ {
			if counts[i] > 0 {
				lr := 1.0 / float64(counts[i]) // Learning rate
				newCentroid := sums[i] / float64(counts[i])
				kmeans.centroids[i].qty = (1.0-lr)*kmeans.centroids[i].qty + lr*newCentroid
			}
		}
	}

	// Assign labels to all points
	labels := make([]int, len(points))
	for i, p := range points {
		labels[i] = kmeans.closestCentroid(p)
	}

	// Stabilize labels by sorting centroids
	centroidIndices := make([]int, kmeans.numClusters)
	for i := range centroidIndices {
		centroidIndices[i] = i
	}

	sort.Slice(centroidIndices, func(i, j int) bool {
		return kmeans.centroids[centroidIndices[i]].qty < kmeans.centroids[centroidIndices[j]].qty
	})

	// Create label mapping
	labelMap := make(map[int]int)
	for newLabel, oldLabel := range centroidIndices {
		labelMap[oldLabel] = newLabel
	}

	// Remap labels
	for i := range labels {
		if newLabel, exists := labelMap[labels[i]]; exists {
			labels[i] = newLabel
		}
	}

	return labels
}

// ClusteredOrder represents an order with its cluster assignment
type ClusteredOrder struct {
	Qty     decimal.Decimal `json:"qty"`
	Cluster int             `json:"cluster"`
}

// ClusterOrderBook applies K-means clustering to an order book
func ClusterOrderBook(orderBook map[string]*OrderQueue, numClusters int, isBid bool) map[string][]*ClusteredOrder {
	kmeansInitMutex.Lock()
	var kmeans *MiniBatchKMeans
	
	if isBid {
		if globalBidKMeans == nil || globalBidKMeans.numClusters != numClusters {
			globalBidKMeans = NewMiniBatchKMeans(numClusters, 1024, 1024)
		}
		kmeans = globalBidKMeans
	} else {
		if globalAskKMeans == nil || globalAskKMeans.numClusters != numClusters {
			globalAskKMeans = NewMiniBatchKMeans(numClusters, 1024, 1024)
		}
		kmeans = globalAskKMeans
	}
	kmeansInitMutex.Unlock()
	
	labels := kmeans.Fit(orderBook)

	clusteredOrders := make(map[string][]*ClusteredOrder)
	labelIdx := 0

	for price, queue := range orderBook {
		queue.mu.RLock()
		orders := make([]*ClusteredOrder, 0, len(queue.orders))
		
		for _, qty := range queue.orders {
			if qty.GreaterThan(decimal.Zero) {
				cluster := 0
				if labelIdx < len(labels) {
					cluster = labels[labelIdx]
					labelIdx++
				}
				
				orders = append(orders, &ClusteredOrder{
					Qty:     qty,
					Cluster: cluster,
				})
			}
		}
		
		if len(orders) > 0 {
			clusteredOrders[price] = orders
		}
		queue.mu.RUnlock()
	}

	return clusteredOrders
}