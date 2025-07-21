class L3OrderBookVisualizer {
	constructor() {
		this.chartContainer = d3.select("#orderbook-chart");
		this.ws = null;
		this.l3Data = null;
		this.chart = null;

		this.initChart();
		this.initWebSocket();
		window.addEventListener("resize", () => this.resizeChart());
	}

	initChart() {
		// Create scales
		this.xScale = d3.scaleLinear().domain([-25, 25]);
		this.yScale = d3.scaleLinear().domain([0, 1000]);

		// Create SVG
		this.svg = this.chartContainer.append('svg')
			.attr('width', '100%')
			.attr('height', '100%');

		// Create plot area group
		this.plotArea = this.svg.append('g')
			.attr('class', 'plot-area');

		this.resizeChart();
	}

	createGradients(selection, bidData, askData) {
		let defs = selection.select('defs');
		if (defs.empty()) {
			defs = selection.append('defs');
		}

		// Remove existing gradients
		defs.selectAll('linearGradient').remove();

		// Create gradients for bids
		bidData.forEach(d => {
			const gradient = defs.append('linearGradient')
				.attr('id', `bid-gradient-${Math.abs(d.index)}`)
				.attr('x1', '0%').attr('y1', '0%')
				.attr('x2', '0%').attr('y2', '100%');
			
			gradient.append('stop')
				.attr('offset', '0%')
				.style('stop-color', '#00ff88');
			
			gradient.append('stop')
				.attr('offset', '100%')
				.style('stop-color', '#004d2a');
		});

		// Create gradients for asks
		askData.forEach(d => {
			const gradient = defs.append('linearGradient')
				.attr('id', `ask-gradient-${d.index}`)
				.attr('x1', '0%').attr('y1', '0%')
				.attr('x2', '0%').attr('y2', '100%');
			
			gradient.append('stop')
				.attr('offset', '0%')
				.style('stop-color', '#ff4444');
			
			gradient.append('stop')
				.attr('offset', '100%')
				.style('stop-color', '#4d0000');
		});
	}

	addPriceLabels(bidData, askData) {
		// Remove existing labels
		this.plotArea.selectAll('.price-labels').remove();

		const labelsGroup = this.plotArea.append('g').attr('class', 'price-labels');
		const labelY = this.yScale.range()[0] + 30;

		// Add bid price labels (every 5th bar)
		bidData.filter((d, i) => i % 5 === 0).forEach(d => {
			const x = this.xScale(d.index);
			const priceText = d.price.toFixed(2);

			// Create diagonal text group
			const textGroup = labelsGroup.append('g')
				.attr('transform', `translate(${x}, ${labelY}) rotate(-45)`);

			// Background rect (adjust for diagonal text)
			textGroup.append('rect')
				.attr('x', -30)
				.attr('y', -8)
				.attr('width', 60)
				.attr('height', 16)
				.style('fill', 'rgba(0, 0, 0, 0.8)')
				.style('stroke', '#00ff88')
				.style('stroke-width', 1);

			// Price text (diagonal)
			textGroup.append('text')
				.attr('x', 0)
				.attr('y', 4)
				.attr('text-anchor', 'middle')
				.style('fill', '#ffffff')
				.style('font-family', 'Monaco, monospace')
				.style('font-size', '11px')
				.style('font-weight', 'bold')
				.text(priceText);
		});

		// Add ask price labels (every 5th bar)
		askData.filter((d, i) => i % 5 === 0).forEach(d => {
			const x = this.xScale(d.index);
			const priceText = d.price.toFixed(2);

			// Create diagonal text group
			const textGroup = labelsGroup.append('g')
				.attr('transform', `translate(${x}, ${labelY}) rotate(45)`);

			// Background rect (adjust for diagonal text)
			textGroup.append('rect')
				.attr('x', -30)
				.attr('y', -8)
				.attr('width', 60)
				.attr('height', 16)
				.style('fill', 'rgba(0, 0, 0, 0.8)')
				.style('stroke', '#ff4444')
				.style('stroke-width', 1);

			// Price text (diagonal)
			textGroup.append('text')
				.attr('x', 0)
				.attr('y', 4)
				.attr('text-anchor', 'middle')
				.style('fill', '#ffffff')
				.style('font-family', 'Monaco, monospace')
				.style('font-size', '11px')
				.style('font-weight', 'bold')
				.text(priceText);
		});
	}

	renderSegmentedBars(allData) {
		// Remove existing bars
		this.plotArea.selectAll('.segmented-bars').remove();

		const barsGroup = this.plotArea.append('g').attr('class', 'segmented-bars');
		const barWidth = Math.min(12, this.xScale.range()[1] / (allData.length * 2)); // Dynamic bar width

		allData.forEach(d => {
			const x = this.xScale(d.index) - barWidth / 2;
			const baseColor = d.index < 0 ? '#00ff88' : '#ff4444';
			const strokeColor = d.index < 0 ? '#00cc66' : '#cc3333';
			
			// Draw individual order segments
			let currentY = this.yScale(0);
			d.orderSizes.forEach((orderSize, segmentIndex) => {
				const segmentHeight = this.yScale(0) - this.yScale(orderSize);
				
				// Vary the color intensity for different segments
				const intensity = Math.max(0.6, 1 - (segmentIndex * 0.1));
				const segmentColor = d.index < 0 
					? `rgba(0, 255, 136, ${intensity})`
					: `rgba(255, 68, 68, ${intensity})`;

				// Draw segment rectangle
				barsGroup.append('rect')
					.attr('x', x)
					.attr('y', currentY - segmentHeight)
					.attr('width', barWidth)
					.attr('height', segmentHeight)
					.style('fill', segmentColor)
					.style('stroke', strokeColor)
					.style('stroke-width', 0.5);

				// Add separator line between segments (except for last segment)
				if (segmentIndex < d.orderSizes.length - 1) {
					barsGroup.append('line')
						.attr('x1', x)
						.attr('x2', x + barWidth)
						.attr('y1', currentY - segmentHeight)
						.attr('y2', currentY - segmentHeight)
						.style('stroke', '#ffffff')
						.style('stroke-width', 1);
				}

				currentY -= segmentHeight;
			});

			// Add outer border for the entire bar
			barsGroup.append('rect')
				.attr('x', x)
				.attr('y', this.yScale(d.size))
				.attr('width', barWidth)
				.attr('height', this.yScale(0) - this.yScale(d.size))
				.style('fill', 'none')
				.style('stroke', strokeColor)
				.style('stroke-width', 1.5);
		});
	}

	addOrderCounts(allData) {
		// Remove existing order counts
		this.plotArea.selectAll('.order-counts').remove();

		const countsGroup = this.plotArea.append('g').attr('class', 'order-counts');

		// Add order count for each bar (only if > 1)
		allData.filter(d => d.orders > 1).forEach(d => {
			const x = this.xScale(d.index);
			const y = this.yScale(d.size) - 15; // Position above the bar

			// Background circle for better visibility
			countsGroup.append('circle')
				.attr('cx', x)
				.attr('cy', y)
				.attr('r', 10)
				.style('fill', 'rgba(0, 0, 0, 0.8)')
				.style('stroke', d.index < 0 ? '#00ff88' : '#ff4444')
				.style('stroke-width', 1);

			// Order count text
			countsGroup.append('text')
				.attr('x', x)
				.attr('y', y + 4) // Center vertically in circle
				.attr('text-anchor', 'middle')
				.style('fill', '#ffff00')
				.style('font-family', 'Monaco, monospace')
				.style('font-size', '10px')
				.style('font-weight', 'bold')
				.text(d.orders);
		});
	}

	resizeChart() {
		if (!this.svg) return;
		
		this.svg
			.style('width', '100%')
			.style('height', '100%');
			
		// Re-render if we have data
		if (this.l3Data) {
			this.renderChart();
		}
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
					this.renderChart();
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

	renderChart() {
		if (!this.svg || !this.l3Data) return;

		const { bids, asks } = this.l3Data;
		if (!bids.length || !asks.length) return;

		// Prepare bid data with individual order segments
		const bidData = bids.slice(0, 25).map((bid, i) => ({
			index: -(i + 1),
			size: Number.parseFloat(bid.total_size),
			price: Number.parseFloat(bid.price),
			orders: bid.order_count,
			orderSizes: bid.orders ? bid.orders.map(o => Number.parseFloat(o)) : [Number.parseFloat(bid.total_size)]
		}));

		// Prepare ask data with individual order segments
		const askData = asks.slice(0, 25).map((ask, i) => ({
			index: i + 1,
			size: Number.parseFloat(ask.total_size),
			price: Number.parseFloat(ask.price),
			orders: ask.order_count,
			orderSizes: ask.orders ? ask.orders.map(o => Number.parseFloat(o)) : [Number.parseFloat(ask.total_size)]
		}));

		// Update y-domain based on max size
		const maxSize = Math.max(
			d3.max(bidData, d => d.size) || 0,
			d3.max(askData, d => d.size) || 0
		);
		this.yScale.domain([0, maxSize * 1.1]);

		// Update scales with container dimensions
		const containerRect = this.chartContainer.node().getBoundingClientRect();
		const margin = { top: 20, right: 20, bottom: 60, left: 20 };
		const width = containerRect.width - margin.left - margin.right;
		const height = containerRect.height - margin.top - margin.bottom;

		this.xScale.range([0, width]);
		this.yScale.range([height, 0]);

		// Position plot area
		this.plotArea.attr('transform', `translate(${margin.left}, ${margin.top})`);

		// Draw center line
		this.plotArea.selectAll('.center-line').remove();
		this.plotArea.append('line')
			.attr('class', 'center-line')
			.attr('x1', this.xScale(0))
			.attr('x2', this.xScale(0))
			.attr('y1', 0)
			.attr('y2', height)
			.style('stroke', '#888')
			.style('stroke-width', 2);

		// Render segmented bars
		this.renderSegmentedBars([...bidData, ...askData]);

		// Add price labels
		this.addPriceLabels(bidData, askData);
		
		// Add order count indicators
		this.addOrderCounts([...bidData, ...askData]);
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
                        <div style="font-size: 12px; color: #00ff88; font-weight: 600;">
                            ${Number.parseFloat(bid.price).toFixed(5)} - ${bid.order_count} orders (${Number.parseFloat(bid.total_size).toFixed(1)} total)
                        </div>
                        <div class="queue-orders" style="margin-top: 4px;">
                            ${bid.orders
								.map((order, orderIndex) => {
									const width = Math.max(
										4,
										(Number.parseFloat(order) /
											Number.parseFloat(bid.max_order)) *
											120,
									);
									const size = Number.parseFloat(order);
									return `<span class="order-bar" title="Order ${orderIndex + 1}: ${size.toFixed(2)}" style="width: ${width}px; background: #00ff88; display: inline-block; height: 8px; margin: 1px; border-radius: 2px;"></span>`;
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
                        <div style="font-size: 12px; color: #ff4444; font-weight: 600;">
                            ${Number.parseFloat(ask.price).toFixed(5)} - ${ask.order_count} orders (${Number.parseFloat(ask.total_size).toFixed(1)} total)
                        </div>
                        <div class="queue-orders" style="margin-top: 4px;">
                            ${ask.orders
								.map((order, orderIndex) => {
									const width = Math.max(
										4,
										(Number.parseFloat(order) /
											Number.parseFloat(ask.max_order)) *
											120,
									);
									const size = Number.parseFloat(order);
									return `<span class="order-bar" title="Order ${orderIndex + 1}: ${size.toFixed(2)}" style="width: ${width}px; background: #ff4444; display: inline-block; height: 8px; margin: 1px; border-radius: 2px;"></span>`;
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