# Binance L3 Order Book Estimator (Go)

A professional-grade real-time L3 order book visualizer for Binance perpetual futures that reconstructs individual order queues from L2 market depth data. Features advanced clustering, precision handling, and interactive visualization.

Built with Claude Code.

Inspired by the Rust implementation by @OctopusTakopi: [binance_l3_est](https://github.com/OctopusTakopi/binance_l3_est)

## ✨ Features

### 🎯 **Core L3 Reconstruction**
- Real-time reconstruction of individual order queues from L2 data
- FIFO-based queue management with intelligent order matching
- Exact decimal arithmetic to prevent floating-point errors
- Support for all Binance perpetual futures symbols

### 🧠 **K-Means Clustering**
- Mini-batch K-means algorithm for grouping orders by quantity
- Configurable cluster count (3-15) with real-time updates
- Reveals market microstructure patterns (institutional vs retail)
- Stable clustering with deterministic initialization

### 🎨 **Advanced Visualization**
- **Age-based coloring**: Darker colors = older orders (front of queue)
- **Cluster-based coloring**: Different colors for each order size cluster  
- **Special highlighting**: Gold colors for largest and second-largest orders
- **Interactive controls**: Toggle clustering, adjust parameters, refresh precision

### 📐 **Precision Handling**
- Automatic fetching of tick size and step size from Binance API
- Dynamic price/quantity formatting based on symbol requirements
- Smart caching with hourly refresh
- Support for all asset classes (BTC, ETH, altcoins, etc.)

### 📊 **Interactive Frontend**
- Real-time D3.js-based stacked bar charts
- Comprehensive order book table view
- Queue visualization with individual order bars
- Symbol switching with live updates
- Responsive design optimized for trading workflows

### ⚡ **Enhanced Queue Management**
- Synthetic order ID tracking for better L3 accuracy
- Order aging and queue optimization
- Multiple removal strategies (FIFO, largest-first, exact-match)
- Comprehensive metrics (average age, partial fills, queue depth)
- Automatic queue maintenance every 30 seconds

## 🚀 Quick Start

### Option 1: Using the run script (Recommended)
```bash
./run.sh ETHUSDT
```

### Option 2: Direct Go command
```bash
go run *.go ethusdt
```

Then open [http://localhost:8080](http://localhost:8080) in your browser.

## 🎮 Controls

- **Symbol Dropdown**: Switch between trading pairs in real-time
- **Clustering Toggle**: Enable/disable K-means clustering
- **Cluster Count**: Adjust number of clusters (3-15)  
- **Color Mode**: Shows current coloring mode (Age-based or Cluster)
- **Precision Refresh**: Force update of symbol precision info

## 📡 WebSocket API

The application exposes a WebSocket API for programmatic control:

```javascript
// Toggle clustering
ws.send(JSON.stringify({
    type: "toggle_kmeans", 
    kmeans_mode: true, 
    num_clusters: 10
}));

// Switch symbol
ws.send(JSON.stringify({
    type: "switch_symbol", 
    symbol: "BTCUSDT"
}));

// Refresh precision
ws.send(JSON.stringify({
    type: "refresh_precision"
}));
```

## 🏗️ Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Binance API   │────│  L3 Reconstruction │────│  Visualization  │
│  (L2 WebSocket) │    │     Algorithm      │    │   (D3.js + Go)  │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                                │
                    ┌──────────────────┐
                    │   K-Means        │
                    │   Clustering     │
                    └──────────────────┘
```

## 🔬 L3 Algorithm Details

The enhanced algorithm uses multiple strategies for accurate queue reconstruction:

1. **Order Addition**: New orders → back of FIFO queue
2. **Order Removal**: 
   - Try exact match first (cancellations)
   - Large changes → remove from biggest orders  
   - Small changes → FIFO removal from front
3. **Queue Maintenance**: Periodic optimization and age updates
4. **Metrics Tracking**: Comprehensive queue analytics

## 🎯 Accuracy

The L3 reconstruction provides realistic estimates by:
- Maintaining FIFO order for realistic queue behavior
- Using multiple removal strategies based on change patterns
- Tracking order ages and partial fills
- Self-optimizing queues to maintain accuracy over time

This approach significantly outperforms simple L2 aggregation for understanding market microstructure.

## 📦 Dependencies

- **Backend**: Go 1.21+, gorilla/websocket, shopspring/decimal
- **Frontend**: D3.js v7, vanilla JavaScript
- **Data Source**: Binance Futures WebSocket API

## 📄 License

MIT License - see [LICENSE](LICENSE) file for details.

---

*This implementation achieves full feature parity with the original Rust version while adding enhanced queue management and WebSocket API controls.* 
