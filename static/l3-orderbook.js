class L3OrderBookVisualizer {
	constructor() {
		this.canvas = document.getElementById("orderbook-canvas");
		this.ctx = this.canvas.getContext("2d");
		this.ws = null;
		this.l3Data = null;

		this.initWebSocket();
		this.resizeCanvas();
		window.addEventListener("resize", () => this.resizeCanvas());
	}

	initWebSocket() {
		const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
		const wsUrl = `${protocol}//${window.location.host}/ws`;

		this.ws = new WebSocket(wsUrl);

		this.ws.onopen = () => {
			document.getElementById("status").textContent = "L3 Connected";
		};

		this.ws.onmessage = (event) => {
			try {
				const message = JSON.parse(event.data);
				if (message.type === "l3_update") {
					this.l3Data = message.data;
					this.render();
					this.updateSidebar();
					this.updateQueueVisualization();
				}
			} catch (error) {
				console.error("Error parsing message:", error);
			}
		};

		this.ws.onclose = () => {
			document.getElementById("status").textContent = "Disconnected";
			setTimeout(() => this.initWebSocket(), 2000);
		};
	}

	resizeCanvas() {
		const rect = this.canvas.parentElement.getBoundingClientRect();
		this.canvas.width = rect.width - 20;
		this.canvas.height = rect.height - 20;
	}

	render() {
		if (!this.l3Data) return;

		this.ctx.clearRect(0, 0, this.canvas.width, this.canvas.height);

		const { bids, asks } = this.l3Data;
		if (!bids.length || !asks.length) return;

		// Find max total size for scaling
		let maxSize = 0;
		[...bids, ...asks].forEach((level) => {
			const size = Number.parseFloat(level.total_size);
			if (size > maxSize) maxSize = size;
		});

		const centerX = this.canvas.width / 2;
		const centerY = this.canvas.height / 2;
		const maxBarHeight = this.canvas.height * 0.4;

		// Draw bids (left side, green)
		bids.slice(0, 50).forEach((bid, index) => {
			const x = centerX - (index + 1) * 6;
			const totalHeight =
				(Number.parseFloat(bid.total_size) / maxSize) * maxBarHeight;

			// Draw individual orders if available
			if (bid.orders && bid.orders.length > 0) {
				let currentY = centerY;
				const orderScale = totalHeight / Number.parseFloat(bid.total_size);

				bid.orders.forEach((orderSize, orderIndex) => {
					const orderHeight = Number.parseFloat(orderSize) * orderScale;
					const intensity = Math.min(255, 100 + orderIndex * 20);

					this.ctx.fillStyle = `rgb(0, ${intensity}, 100)`;
					this.ctx.fillRect(x, currentY - orderHeight, 5, orderHeight);
					currentY -= orderHeight;
				});
			} else {
				// Fallback to single bar
				this.ctx.fillStyle = "#00ff88";
				this.ctx.fillRect(x, centerY - totalHeight, 5, totalHeight);
			}

			// Draw order count indicator
			if (bid.order_count > 1) {
				this.ctx.fillStyle = "#ffff00";
				this.ctx.font = "8px Monaco";
				this.ctx.fillText(bid.order_count.toString(), x, centerY + 15);
			}
		});

		// Draw asks (right side, red)
		asks.slice(0, 50).forEach((ask, index) => {
			const x = centerX + index * 6;
			const totalHeight =
				(Number.parseFloat(ask.total_size) / maxSize) * maxBarHeight;

			// Draw individual orders if available
			if (ask.orders && ask.orders.length > 0) {
				let currentY = centerY;
				const orderScale = totalHeight / Number.parseFloat(ask.total_size);

				ask.orders.forEach((orderSize, orderIndex) => {
					const orderHeight = Number.parseFloat(orderSize) * orderScale;
					const intensity = Math.min(255, 100 + orderIndex * 20);

					this.ctx.fillStyle = `rgb(${intensity}, 0, 100)`;
					this.ctx.fillRect(x, currentY - orderHeight, 5, orderHeight);
					currentY -= orderHeight;
				});
			} else {
				// Fallback to single bar
				this.ctx.fillStyle = "#ff4444";
				this.ctx.fillRect(x, centerY - totalHeight, 5, totalHeight);
			}

			// Draw order count indicator
			if (ask.order_count > 1) {
				this.ctx.fillStyle = "#ffff00";
				this.ctx.font = "8px Monaco";
				this.ctx.fillText(ask.order_count.toString(), x, centerY + 15);
			}
		});

		// Draw center line
		this.ctx.strokeStyle = "#666";
		this.ctx.beginPath();
		this.ctx.moveTo(centerX, 0);
		this.ctx.lineTo(centerX, this.canvas.height);
		this.ctx.stroke();
	}

	updateSidebar() {
		if (!this.l3Data) return;

		const { bids, asks } = this.l3Data;

		// Update stats
		const totalBidOrders = bids.reduce(
			(sum, level) => sum + level.order_count,
			0,
		);
		const totalAskOrders = asks.reduce(
			(sum, level) => sum + level.order_count,
			0,
		);

		document.getElementById("book-stats").innerHTML = `
            Levels: ${bids.length} bids, ${asks.length} asks<br>
            Orders: ${totalBidOrders} bids, ${totalAskOrders} asks
        `;

		// Update asks
		document.getElementById("asks-section").innerHTML = `
            <h4 style="color: #ff4444;">Asks</h4>
            ${asks
							.slice(0, 15)
							.reverse()
							.map(
								(ask) => `
                <div class="level ask-level">
                    <span>${Number.parseFloat(ask.price).toFixed(5)}</span>
                    <span>${Number.parseFloat(ask.total_size).toFixed(0)}</span>
                    <span>(${ask.order_count})</span>
                </div>
            `,
							)
							.join("")}
        `;

		// Update bids
		document.getElementById("bids-section").innerHTML = `
            <h4 style="color: #00ff88;">Bids</h4>
            ${bids
							.slice(0, 15)
							.map(
								(bid) => `
                <div class="level bid-level">
                    <span>${Number.parseFloat(bid.price).toFixed(5)}</span>
                    <span>${Number.parseFloat(bid.total_size).toFixed(0)}</span>
                    <span>(${bid.order_count})</span>
                </div>
            `,
							)
							.join("")}
        `;

		// Update spread
		if (bids.length > 0 && asks.length > 0) {
			const spread =
				Number.parseFloat(asks[0].price) - Number.parseFloat(bids[0].price);
			const spreadPct = (spread / Number.parseFloat(asks[0].price)) * 100;
			document.getElementById("spread-info").innerHTML = `
                Spread: ${spread.toFixed(5)} (${spreadPct.toFixed(3)}%)
            `;
		}
	}

	updateQueueVisualization() {
		if (!this.l3Data) return;

		const { bids, asks } = this.l3Data;
		const queueDisplay = document.getElementById("queue-display");

		// Show detailed queue for top 5 levels of each side
		let html = '<h4 style="color: #00ff88;">Top Bid Queues</h4>';
		bids.slice(0, 5).forEach((bid, levelIndex) => {
			if (bid.orders && bid.orders.length > 0) {
				html += `
                    <div style="margin: 5px 0; border-left: 3px solid #00ff88; padding-left: 8px;">
                        <div style="font-size: 10px; color: #00ff88;">
                            ${Number.parseFloat(bid.price).toFixed(5)} - ${bid.order_count} orders
                        </div>
                        <div class="queue-orders">
                            ${bid.orders
															.map((order, orderIndex) => {
																const width = Math.max(
																	2,
																	(Number.parseFloat(order) /
																		Number.parseFloat(bid.max_order)) *
																		100,
																);
																return `<span class="order-bar" style="width: ${width}px; background: #00ff88;"></span>`;
															})
															.join("")}
                        </div>
                    </div>
                `;
			}
		});

		html += '<h4 style="color: #ff4444; margin-top: 15px;">Top Ask Queues</h4>';
		asks.slice(0, 5).forEach((ask, levelIndex) => {
			if (ask.orders && ask.orders.length > 0) {
				html += `
                    <div style="margin: 5px 0; border-left: 3px solid #ff4444; padding-left: 8px;">
                        <div style="font-size: 10px; color: #ff4444;">
                            ${Number.parseFloat(ask.price).toFixed(5)} - ${ask.order_count} orders
                        </div>
                        <div class="queue-orders">
                            ${ask.orders
															.map((order, orderIndex) => {
																const width = Math.max(
																	2,
																	(Number.parseFloat(order) /
																		Number.parseFloat(ask.max_order)) *
																		100,
																);
																return `<span class="order-bar" style="width: ${width}px; background: #ff4444;"></span>`;
															})
															.join("")}
                        </div>
                    </div>
                `;
			}
		});

		queueDisplay.innerHTML = html;

		// Update queue stats
		const avgBidQueueSize =
			bids.length > 0
				? bids.reduce((sum, b) => sum + b.order_count, 0) / bids.length
				: 0;
		const avgAskQueueSize =
			asks.length > 0
				? asks.reduce((sum, a) => sum + a.order_count, 0) / asks.length
				: 0;

		document.getElementById("queue-stats").innerHTML = `
            Avg Queue Size: ${avgBidQueueSize.toFixed(1)} bids, ${avgAskQueueSize.toFixed(1)} asks
        `;
	}
}

// Initialize when page loads
document.addEventListener("DOMContentLoaded", () => {
	new L3OrderBookVisualizer();
});
