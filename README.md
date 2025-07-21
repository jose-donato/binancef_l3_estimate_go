# Binance L3 Order Book Estimator (Go)

A real-time L3 order book visualizer for Binance perpetual futures that reconstructs individual order queues from L2 market depth data using go + d3fc.

Built with Claude Sonnet.

Inspired by the Rust implementation by @OctopusTakopi: [binance_l3_est](https://github.com/OctopusTakopi/binance_l3_est).

## Features

- **Real-time L3 Reconstruction**: Estimates individual orders within each price level using FIFO queue simulation
- **Dynamic Symbol Switching**: Switch between major crypto pairs (BTC, ETH, SOL, etc.) without restarting
- **Advanced Visualization**: Segmented bars showing individual orders with d3fc charting
- **Order Queue Display**: Visual representation of FIFO queues for top price levels

## L3 Algorithm

The core algorithm reconstructs Level 3 order queues from Level 2 updates:

1. **Quantity Increase**: New orders are added to the back of the FIFO queue
2. **Quantity Decrease**: First attempts exact order matching for cancellations, then reduces largest order as fallback
3. **FIFO Maintenance**: Preserves realistic order sequence and timing
4. **Decimal Precision**: Uses exact decimal arithmetic to prevent floating-point errors

This approach provides a realistic estimation of individual order sizes and queue positions within each price level.

## Usage

1. **Run the server**:
   ```sh
   go run main.go [symbol]
   ```
   Defaults to `ethusdt` if no symbol provided.

2. **Open visualization**:
   Navigate to `http://localhost:8080`

3. **Switch symbols**:
   Use the dropdown in the top-left to change trading pairs dynamically 