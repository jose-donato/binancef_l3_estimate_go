# Binance L3 Order Book Estimator (Go)

This project is a quick experiment to create a real-time L3 order book estimator for Binance perpetual futures, written in Go. It reconstructs an L3 view from L2 market depth data and visualizes it on a simple web interface.

This was built with Claude Sonnet.

Inspired by the Rust implementation by @OctopusTakopi: [binance_l3_est](https://github.com/OctopusTakopi/binance_l3_est).

## How It Works

1.  **Fetches L2 Snapshot**: It starts by getting the initial order book snapshot from the Binance API.
2.  **Connects to WebSocket**: It subscribes to the real-time L2 depth stream for a given symbol.
3.  **Reconstructs L3 Queues**: It applies a simple algorithm to the L2 updates to estimate the individual orders that make up the total quantity at each price level.
4.  **Serves Data**: A WebSocket server sends the reconstructed L3 data to the frontend for visualization.
5.  **Visualizes**: A simple HTML/JS frontend renders the order book.

## Usage

1.  **Run the server**:
    ```sh
    go run main.go [symbol]
    ```
    If no symbol is provided, it defaults to `dogeusdt`.

    Example:
    ```sh
    go run main.go btcusdt
    ```

2.  **Open the visualization**:
    Navigate to `http://localhost:8080` in your web browser. 