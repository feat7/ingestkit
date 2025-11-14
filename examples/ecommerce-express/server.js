const express = require('express');
const bodyParser = require('body-parser');
const { v4: uuidv4 } = require('uuid');
const { Client } = require('./ingestkit');

const app = express();
const PORT = process.env.PORT || 3000;

// Initialize IngestKit client
const analytics = new Client();

// Middleware
app.use(bodyParser.json());

// In-memory data (for demo purposes)
const products = [
  { id: 'prod_001', name: 'Wireless Headphones', category: 'Electronics', price: 79.99 },
  { id: 'prod_002', name: 'Running Shoes', category: 'Sports', price: 129.99 },
  { id: 'prod_003', name: 'Coffee Maker', category: 'Home', price: 49.99 },
  { id: 'prod_004', name: 'Laptop Stand', category: 'Electronics', price: 39.99 },
  { id: 'prod_005', name: 'Yoga Mat', category: 'Sports', price: 29.99 },
];

const carts = {};

// API Routes

// GET /products - List all products
app.get('/products', (req, res) => {
  res.json({ products });
});

// GET /products/:id - View a product (tracks product_viewed event)
app.get('/products/:id', async (req, res) => {
  const { id } = req.params;
  const product = products.find(p => p.id === id);

  if (!product) {
    return res.status(404).json({ error: 'Product not found' });
  }

  // Track product view event
  const userId = req.headers['x-user-id'] || 'guest';
  const sessionId = req.headers['x-session-id'] || uuidv4();

  try {
    await analytics.productViewed.send({
      userId,
      sessionId,
      productId: product.id,
      productName: product.name,
      productCategory: product.category,
      price: product.price.toString(),
      currency: 'USD',
      source: req.query.source || 'direct',
      metadata: {
        userAgent: req.headers['user-agent'],
        referer: req.headers['referer'],
      },
    });

    console.log(`✓ Tracked product view: ${product.name} (user: ${userId})`);
  } catch (error) {
    console.error('Failed to track product view:', error.message);
  }

  res.json({ product });
});

// POST /cart/add - Add item to cart (tracks added_to_cart event)
app.post('/cart/add', async (req, res) => {
  const { productId, quantity = 1 } = req.body;
  const userId = req.headers['x-user-id'] || 'guest';
  const sessionId = req.headers['x-session-id'] || uuidv4();

  const product = products.find(p => p.id === productId);
  if (!product) {
    return res.status(404).json({ error: 'Product not found' });
  }

  // Update cart
  if (!carts[userId]) {
    carts[userId] = [];
  }

  const existing = carts[userId].find(item => item.productId === productId);
  if (existing) {
    existing.quantity += quantity;
  } else {
    carts[userId].push({ productId, quantity, price: product.price });
  }

  const cartTotal = carts[userId].reduce((sum, item) => sum + (item.price * item.quantity), 0);

  // Track add to cart event
  try {
    await analytics.addedToCart.send({
      userId,
      sessionId,
      productId: product.id,
      productName: product.name,
      quantity,
      price: product.price.toString(),
      currency: 'USD',
      cartTotal: cartTotal.toString(),
      metadata: {
        cartItemCount: carts[userId].length,
      },
    });

    console.log(`✓ Tracked add to cart: ${product.name} x${quantity} (user: ${userId})`);
  } catch (error) {
    console.error('Failed to track add to cart:', error.message);
  }

  res.json({ success: true, cart: carts[userId], cartTotal });
});

// POST /checkout/start - Start checkout (tracks checkout_started event)
app.post('/checkout/start', async (req, res) => {
  const userId = req.headers['x-user-id'] || 'guest';
  const sessionId = req.headers['x-session-id'] || uuidv4();

  const cart = carts[userId] || [];
  if (cart.length === 0) {
    return res.status(400).json({ error: 'Cart is empty' });
  }

  const cartId = uuidv4();
  const cartTotal = cart.reduce((sum, item) => sum + (item.price * item.quantity), 0);
  const numItems = cart.reduce((sum, item) => sum + item.quantity, 0);

  // Enrich cart items with product details
  const items = cart.map(item => {
    const product = products.find(p => p.id === item.productId);
    return {
      productId: item.productId,
      productName: product.name,
      quantity: item.quantity,
      price: item.price,
    };
  });

  // Track checkout started event
  try {
    await analytics.checkoutStarted.send({
      userId,
      sessionId,
      cartId,
      numItems,
      cartTotal: cartTotal.toString(),
      currency: 'USD',
      items,
    });

    console.log(`✓ Tracked checkout started: ${numItems} items, $${cartTotal.toFixed(2)} (user: ${userId})`);
  } catch (error) {
    console.error('Failed to track checkout started:', error.message);
  }

  res.json({ success: true, cartId, items, cartTotal });
});

// POST /checkout/complete - Complete order (tracks order_completed event)
app.post('/checkout/complete', async (req, res) => {
  const { cartId, paymentMethod, shippingAddress, discountCode } = req.body;
  const userId = req.headers['x-user-id'] || 'guest';
  const sessionId = req.headers['x-session-id'] || uuidv4();

  const cart = carts[userId] || [];
  if (cart.length === 0) {
    return res.status(400).json({ error: 'Cart is empty' });
  }

  const orderId = uuidv4();
  const totalAmount = cart.reduce((sum, item) => sum + (item.price * item.quantity), 0);
  const numItems = cart.reduce((sum, item) => sum + item.quantity, 0);

  // Apply discount if provided (simple 10% discount for demo)
  let discountAmount = 0;
  if (discountCode === 'SAVE10') {
    discountAmount = totalAmount * 0.1;
  }

  const finalAmount = totalAmount - discountAmount;

  // Enrich cart items with product details
  const items = cart.map(item => {
    const product = products.find(p => p.id === item.productId);
    return {
      productId: item.productId,
      productName: product.name,
      quantity: item.quantity,
      price: item.price,
    };
  });

  // Track order completed event
  try {
    await analytics.orderCompleted.send({
      userId,
      sessionId,
      orderId,
      cartId: cartId || uuidv4(),
      totalAmount: finalAmount.toString(),
      currency: 'USD',
      paymentMethod,
      numItems,
      shippingAddress: shippingAddress || {
        street: '123 Main St',
        city: 'San Francisco',
        state: 'CA',
        zip: '94102',
        country: 'US',
      },
      items,
      discountCode: discountCode || null,
      discountAmount: discountAmount > 0 ? discountAmount.toString() : null,
      metadata: {
        originalAmount: totalAmount,
      },
    });

    console.log(`✓ Tracked order completed: Order ${orderId}, $${finalAmount.toFixed(2)} (user: ${userId})`);
  } catch (error) {
    console.error('Failed to track order completed:', error.message);
  }

  // Clear cart
  carts[userId] = [];

  res.json({ success: true, orderId, totalAmount: finalAmount, items });
});

// GET /cart - View cart
app.get('/cart', (req, res) => {
  const userId = req.headers['x-user-id'] || 'guest';
  const cart = carts[userId] || [];

  const items = cart.map(item => {
    const product = products.find(p => p.id === item.productId);
    return {
      ...item,
      productName: product.name,
      subtotal: item.price * item.quantity,
    };
  });

  const total = items.reduce((sum, item) => sum + item.subtotal, 0);

  res.json({ items, total });
});

// Health check
app.get('/health', (req, res) => {
  res.json({ status: 'ok' });
});

// Start server
app.listen(PORT, () => {
  console.log(`🚀 E-commerce API running on http://localhost:${PORT}`);
  console.log(`📊 IngestKit analytics enabled`);
  console.log();
  console.log('Example requests:');
  console.log(`  curl http://localhost:${PORT}/products`);
  console.log(`  curl http://localhost:${PORT}/products/prod_001`);
  console.log(`  curl -X POST http://localhost:${PORT}/cart/add -H "Content-Type: application/json" -d '{"productId":"prod_001","quantity":1}'`);
});
