# SOLID Refactoring Examples

## Example 1: E-commerce Order System (Multiple Violations)

### Before: Monolithic OrderProcessor

```typescript
// order-processor.ts (450 lines - SRP, OCP, DIP violations)
class OrderProcessor {
  private db = new MySQLDatabase();
  private stripe = new StripePaymentGateway();
  private sendgrid = new SendGridMailer();
  private fedex = new FedExShipping();

  async processOrder(orderData: any) {
    // Validation (mixed with processing)
    if (!orderData.items || orderData.items.length === 0) {
      throw new Error('No items');
    }
    if (!orderData.customer.email) {
      throw new Error('No email');
    }

    // Calculate totals (business logic)
    let subtotal = 0;
    for (const item of orderData.items) {
      subtotal += item.price * item.quantity;
    }

    // Apply discounts (hardcoded strategies)
    let discount = 0;
    if (orderData.couponCode === 'SAVE10') {
      discount = subtotal * 0.1;
    } else if (orderData.couponCode === 'SAVE20') {
      discount = subtotal * 0.2;
    } else if (orderData.customer.isPremium) {
      discount = subtotal * 0.05;
    }

    const total = subtotal - discount;

    // Calculate shipping (hardcoded carriers)
    let shippingCost = 0;
    if (orderData.shippingMethod === 'fedex') {
      shippingCost = await this.fedex.calculateRate(orderData);
    } else if (orderData.shippingMethod === 'ups') {
      // Would need UPS integration...
    }

    // Process payment (direct dependency)
    const paymentResult = await this.stripe.charge({
      amount: total + shippingCost,
      customerId: orderData.customer.stripeId
    });

    // Save to database (direct dependency)
    await this.db.query(`INSERT INTO orders ...`);

    // Send notifications (direct dependency)
    await this.sendgrid.send({
      to: orderData.customer.email,
      subject: 'Order Confirmed',
      body: `Your order total: $${total + shippingCost}`
    });

    return { orderId: paymentResult.id, total: total + shippingCost };
  }
}
```

### After: SOLID-Compliant Design

```typescript
// domain/order.ts - Pure domain model
interface OrderItem {
  productId: string;
  price: number;
  quantity: number;
}

interface Order {
  id: string;
  items: OrderItem[];
  customerId: string;
  couponCode?: string;
  shippingMethod: string;
  subtotal: number;
  discount: number;
  shippingCost: number;
  total: number;
}

// domain/discount-strategy.ts - OCP: Open for new discount types
interface DiscountStrategy {
  calculate(subtotal: number, context: DiscountContext): number;
}

class PercentageDiscount implements DiscountStrategy {
  constructor(private percentage: number) {}

  calculate(subtotal: number): number {
    return subtotal * this.percentage;
  }
}

class PremiumCustomerDiscount implements DiscountStrategy {
  calculate(subtotal: number, context: DiscountContext): number {
    return context.isPremiumCustomer ? subtotal * 0.05 : 0;
  }
}

// domain/discount-resolver.ts
class DiscountResolver {
  private strategies: Map<string, DiscountStrategy> = new Map([
    ['SAVE10', new PercentageDiscount(0.1)],
    ['SAVE20', new PercentageDiscount(0.2)],
  ]);

  resolve(couponCode?: string, isPremium?: boolean): DiscountStrategy {
    if (couponCode && this.strategies.has(couponCode)) {
      return this.strategies.get(couponCode)!;
    }
    if (isPremium) {
      return new PremiumCustomerDiscount();
    }
    return new PercentageDiscount(0);
  }
}

// ports/payment-gateway.ts - DIP: Abstraction
interface PaymentGateway {
  charge(amount: number, customerId: string): Promise<PaymentResult>;
}

// ports/shipping-calculator.ts - DIP: Abstraction
interface ShippingCalculator {
  calculateRate(order: Order): Promise<number>;
}

// ports/order-repository.ts - DIP: Abstraction
interface OrderRepository {
  save(order: Order): Promise<void>;
}

// ports/notification-service.ts - DIP: Abstraction
interface NotificationService {
  sendOrderConfirmation(order: Order, email: string): Promise<void>;
}

// application/order-validator.ts - SRP: Only validation
class OrderValidator {
  validate(orderData: CreateOrderDTO): ValidationResult {
    const errors: string[] = [];

    if (!orderData.items?.length) {
      errors.push('Order must have at least one item');
    }
    if (!orderData.customerEmail) {
      errors.push('Customer email is required');
    }

    return {
      isValid: errors.length === 0,
      errors
    };
  }
}

// application/order-calculator.ts - SRP: Only calculations
class OrderCalculator {
  constructor(private discountResolver: DiscountResolver) {}

  calculate(items: OrderItem[], couponCode?: string, isPremium?: boolean): OrderTotals {
    const subtotal = items.reduce(
      (sum, item) => sum + item.price * item.quantity,
      0
    );

    const strategy = this.discountResolver.resolve(couponCode, isPremium);
    const discount = strategy.calculate(subtotal, { isPremiumCustomer: isPremium });

    return {
      subtotal,
      discount,
      total: subtotal - discount
    };
  }
}

// application/order-service.ts - SRP: Orchestration only
class OrderService {
  constructor(
    private validator: OrderValidator,
    private calculator: OrderCalculator,
    private paymentGateway: PaymentGateway,
    private shippingCalculator: ShippingCalculator,
    private orderRepository: OrderRepository,
    private notificationService: NotificationService
  ) {}

  async processOrder(orderData: CreateOrderDTO): Promise<OrderResult> {
    // Validate
    const validation = this.validator.validate(orderData);
    if (!validation.isValid) {
      throw new ValidationError(validation.errors);
    }

    // Calculate
    const totals = this.calculator.calculate(
      orderData.items,
      orderData.couponCode,
      orderData.isPremiumCustomer
    );

    // Shipping
    const shippingCost = await this.shippingCalculator.calculateRate(orderData);

    // Payment
    const finalTotal = totals.total + shippingCost;
    const payment = await this.paymentGateway.charge(
      finalTotal,
      orderData.customerId
    );

    // Create order
    const order: Order = {
      id: payment.orderId,
      items: orderData.items,
      customerId: orderData.customerId,
      ...totals,
      shippingCost,
      total: finalTotal
    };

    // Persist
    await this.orderRepository.save(order);

    // Notify
    await this.notificationService.sendOrderConfirmation(
      order,
      orderData.customerEmail
    );

    return { orderId: order.id, total: finalTotal };
  }
}

// infrastructure/stripe-payment-gateway.ts - DIP: Implementation
class StripePaymentGateway implements PaymentGateway {
  constructor(private stripe: Stripe) {}

  async charge(amount: number, customerId: string): Promise<PaymentResult> {
    const result = await this.stripe.charges.create({
      amount: Math.round(amount * 100),
      currency: 'usd',
      customer: customerId
    });
    return { orderId: result.id, success: true };
  }
}

// Dependency injection setup
const orderService = new OrderService(
  new OrderValidator(),
  new OrderCalculator(new DiscountResolver()),
  new StripePaymentGateway(stripe),
  new FedExShippingCalculator(fedexClient),
  new PostgresOrderRepository(db),
  new SendGridNotificationService(sendgrid)
);
```

---

## Example 2: Report Generator (ISP Violation)

### Before

```java
// BAD: All report types must implement all methods
interface ReportGenerator {
    void generatePDF();
    void generateExcel();
    void generateCSV();
    void generateHTML();
    void sendByEmail(String recipient);
    void uploadToS3(String bucket);
    void print();
}

class SalesReport implements ReportGenerator {
    public void generatePDF() { /* works */ }
    public void generateExcel() { /* works */ }
    public void generateCSV() { throw new UnsupportedOperationException(); }
    public void generateHTML() { throw new UnsupportedOperationException(); }
    public void sendByEmail(String r) { /* works */ }
    public void uploadToS3(String b) { throw new UnsupportedOperationException(); }
    public void print() { throw new UnsupportedOperationException(); }
}
```

### After

```java
// GOOD: Segregated interfaces
interface PDFExportable {
    byte[] toPDF();
}

interface ExcelExportable {
    byte[] toExcel();
}

interface Emailable {
    void sendTo(String recipient);
}

interface CloudStorable {
    void uploadTo(CloudStorage storage);
}

class SalesReport implements PDFExportable, ExcelExportable, Emailable {
    public byte[] toPDF() { /* implementation */ }
    public byte[] toExcel() { /* implementation */ }
    public void sendTo(String recipient) { /* implementation */ }
}

class InternalMemo implements PDFExportable {
    public byte[] toPDF() { /* implementation */ }
}
```

---

## Benefits Summary

| Before | After |
|--------|-------|
| 450 lines in one file | 10-50 lines per file |
| 1 class, 5 responsibilities | 8+ classes, 1 responsibility each |
| Hardcoded dependencies | Injected abstractions |
| Adding features = modifying code | Adding features = adding classes |
| Untestable | Fully unit-testable |
