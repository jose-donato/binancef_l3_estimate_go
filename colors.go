package main

import (
	"fmt"
	"math"

	"github.com/shopspring/decimal"
)

// Color represents an RGB color
type Color struct {
	R, G, B uint8
}

// ToHex converts the color to a hex string
func (c Color) ToHex() string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

// Color palettes matching the Rust implementation
var (
	// BID_COLORS - Blue gradient palette for bid orders (darker = older/front of queue)
	BidColors = []Color{
		{222, 235, 247}, // Light Blue
		{204, 227, 245}, // Lighter Blue
		{158, 202, 225}, // Blue
		{129, 189, 231}, // Light Medium Blue
		{107, 174, 214}, // Medium Blue
		{78, 157, 202},  // Medium Deep Blue
		{49, 130, 189},  // Deep Blue
		{33, 113, 181},  // Darker Deep Blue
		{16, 96, 168},   // Dark Blue
		{8, 81, 156},    // Darkest Blue
	}

	// ASK_COLORS - Orange/Red gradient palette for ask orders (darker = older/front of queue)
	AskColors = []Color{
		{254, 230, 206}, // Light Orange
		{253, 216, 186}, // Lighter Orange
		{253, 174, 107}, // Orange
		{253, 159, 88},  // Light Deep Orange
		{253, 141, 60},  // Deep Orange
		{245, 126, 47},  // Medium Red-Orange
		{230, 85, 13},   // Red-Orange
		{204, 75, 12},   // Darker Red-Orange
		{179, 65, 10},   // Dark Red
		{166, 54, 3},    // Darkest Red
	}

	// Special colors for highlighting
	GoldColor       = Color{255, 215, 0}   // Gold for largest order
	DarkGoldColor   = Color{184, 134, 11}  // Dark gold for second largest
	DefaultBidColor = Color{0, 128, 0}     // Dark green fallback
	DefaultAskColor = Color{139, 0, 0}     // Dark red fallback
)

// GetOrderAgeColor returns a color based on the order's position in the queue (age)
// index: position in the order queue (0 = front/oldest, higher = newer)
// isBid: true for bid orders, false for ask orders
func GetOrderAgeColor(index int, isBid bool) Color {
	var palette []Color
	if isBid {
		palette = BidColors
	} else {
		palette = AskColors
	}

	// Map index to color palette
	if index < len(palette) {
		return palette[index]
	}

	// For orders beyond the palette size, use the darkest color
	return palette[len(palette)-1]
}

// GetClusterColor returns a color for a specific cluster
func GetClusterColor(cluster int, isBid bool) Color {
	var palette []Color
	if isBid {
		palette = BidColors
	} else {
		palette = AskColors
	}

	// Cycle through the palette for cluster colors
	colorIndex := cluster % len(palette)
	return palette[colorIndex]
}

// GetSpecialOrderColor returns special colors for highlighted orders
func GetSpecialOrderColor(orderQty, maxOrder, secondMaxOrder decimal.Decimal) *Color {
	if orderQty.Equal(maxOrder) {
		return &GoldColor
	}
	if orderQty.Equal(secondMaxOrder) {
		return &DarkGoldColor
	}
	return nil // No special color
}

// InterpolateColor creates a color between two colors based on a factor (0.0 to 1.0)
func InterpolateColor(color1, color2 Color, factor float64) Color {
	factor = math.Max(0, math.Min(1, factor)) // Clamp to [0,1]
	
	r := uint8(float64(color1.R) + factor*float64(int(color2.R)-int(color1.R)))
	g := uint8(float64(color1.G) + factor*float64(int(color2.G)-int(color1.G)))
	b := uint8(float64(color1.B) + factor*float64(int(color2.B)-int(color1.B)))
	
	return Color{R: r, G: g, B: b}
}

// BrightenColor brightens a color by a factor (multiplier > 1.0 brightens)
func BrightenColor(color Color, factor float32) Color {
	r := uint8(math.Min(255, float64(color.R)*float64(factor)))
	g := uint8(math.Min(255, float64(color.G)*float64(factor)))
	b := uint8(math.Min(255, float64(color.B)*float64(factor)))
	
	return Color{R: r, G: g, B: b}
}

// GenerateOrderColors generates colors for all orders in a price level
func GenerateOrderColors(orders []decimal.Decimal, isBid bool, maxOrder, secondMaxOrder decimal.Decimal) []string {
	colors := make([]string, len(orders))
	
	for i, order := range orders {
		// Check for special highlighting first
		if specialColor := GetSpecialOrderColor(order, maxOrder, secondMaxOrder); specialColor != nil {
			colors[i] = specialColor.ToHex()
			continue
		}
		
		// Use age-based coloring (position in queue determines color)
		color := GetOrderAgeColor(i, isBid)
		colors[i] = color.ToHex()
	}
	
	return colors
}

// GenerateClusteredOrderColors generates colors for clustered orders
func GenerateClusteredOrderColors(clusteredOrders []*ClusteredOrder, isBid bool, maxOrder, secondMaxOrder decimal.Decimal) []string {
	colors := make([]string, len(clusteredOrders))
	
	for i, order := range clusteredOrders {
		// Check for special highlighting first
		if specialColor := GetSpecialOrderColor(order.Qty, maxOrder, secondMaxOrder); specialColor != nil {
			colors[i] = specialColor.ToHex()
			continue
		}
		
		// Use cluster-based coloring
		color := GetClusterColor(order.Cluster, isBid)
		colors[i] = color.ToHex()
	}
	
	return colors
}