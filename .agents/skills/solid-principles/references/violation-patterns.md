# SOLID Violation Detection Patterns

## Single Responsibility Principle (SRP) Violations

### Pattern 1: God Class

**Indicators**:
- File size > 300 lines
- More than 10 public methods
- Multiple unrelated data fields
- Class handles I/O, business logic, and data persistence

**Detection Regex**:
```regex
# Find classes with many methods (TypeScript/Java)
class\s+\w+\s*\{[\s\S]{3000,}\}
```

**Example Violation** (TypeScript):
```typescript
// BAD: UserManager does too much
class UserManager {
  // User CRUD
  createUser(data: UserData) { /* DB operations */ }
  updateUser(id: string, data: UserData) { /* DB operations */ }
  deleteUser(id: string) { /* DB operations */ }

  // Authentication
  login(email: string, password: string) { /* Auth logic */ }
  logout(userId: string) { /* Session handling */ }
  resetPassword(email: string) { /* Email + DB */ }

  // Notifications
  sendWelcomeEmail(user: User) { /* Email sending */ }
  sendPasswordResetEmail(user: User) { /* Email sending */ }

  // Reporting
  generateUserReport() { /* Report generation */ }
  exportToCSV() { /* File generation */ }
}
```

**Refactored**:
```typescript
// GOOD: Separated concerns
class UserRepository {
  create(data: UserData): Promise<User> { }
  update(id: string, data: UserData): Promise<User> { }
  delete(id: string): Promise<void> { }
}

class AuthenticationService {
  constructor(private userRepo: UserRepository) { }
  login(email: string, password: string): Promise<Session> { }
  logout(sessionId: string): Promise<void> { }
}

class UserNotificationService {
  sendWelcomeEmail(user: User): Promise<void> { }
  sendPasswordResetEmail(user: User): Promise<void> { }
}

class UserReportService {
  generateReport(): Promise<Report> { }
  exportToCSV(): Promise<Buffer> { }
}
```

### Pattern 2: Mixed Abstraction Levels

**Indicators**:
- Method contains both high-level orchestration and low-level details
- Single method handles HTTP, business logic, and database

**Example Violation** (Python):
```python
# BAD: Mixed abstraction levels
def process_order(request):
    # Low-level: Parse HTTP request
    order_data = json.loads(request.body)

    # Business logic
    total = sum(item['price'] * item['qty'] for item in order_data['items'])
    if total > 1000:
        discount = total * 0.1
        total -= discount

    # Low-level: Database operations
    conn = psycopg2.connect(DATABASE_URL)
    cursor = conn.cursor()
    cursor.execute("INSERT INTO orders ...")
    conn.commit()

    # Low-level: Send email
    smtp = smtplib.SMTP('localhost')
    smtp.send_message(create_order_email(order_data))

    return JsonResponse({'status': 'ok'})
```

**Refactored**:
```python
# GOOD: Single abstraction level per function
def process_order(request):
    order_data = parse_order_request(request)
    order = calculate_order_totals(order_data)
    saved_order = save_order(order)
    notify_order_placed(saved_order)
    return create_success_response(saved_order)
```

---

## Open/Closed Principle (OCP) Violations

### Pattern 1: Type Switching

**Indicators**:
- Switch/if-else on type field
- Adding new types requires modifying existing code

**Example Violation** (Java):
```java
// BAD: Must modify for each new shape
public class AreaCalculator {
    public double calculate(Shape shape) {
        if (shape.type.equals("circle")) {
            return Math.PI * shape.radius * shape.radius;
        } else if (shape.type.equals("rectangle")) {
            return shape.width * shape.height;
        } else if (shape.type.equals("triangle")) {
            return 0.5 * shape.base * shape.height;
        }
        throw new IllegalArgumentException("Unknown shape");
    }
}
```

**Refactored**:
```java
// GOOD: Open for extension, closed for modification
public interface Shape {
    double calculateArea();
}

public class Circle implements Shape {
    private final double radius;

    @Override
    public double calculateArea() {
        return Math.PI * radius * radius;
    }
}

public class Rectangle implements Shape {
    private final double width, height;

    @Override
    public double calculateArea() {
        return width * height;
    }
}
```

### Pattern 2: Hardcoded Strategies

**Indicators**:
- Algorithm variations in if/else blocks
- Configuration changes require code changes

---

## Liskov Substitution Principle (LSP) Violations

### Pattern 1: Throwing Unexpected Exceptions

**Example Violation**:
```typescript
// BAD: Square breaks Rectangle contract
class Rectangle {
  constructor(protected width: number, protected height: number) {}

  setWidth(w: number) { this.width = w; }
  setHeight(h: number) { this.height = h; }
  getArea(): number { return this.width * this.height; }
}

class Square extends Rectangle {
  setWidth(w: number) {
    this.width = w;
    this.height = w; // Violates expected behavior
  }

  setHeight(h: number) {
    this.height = h;
    this.width = h; // Violates expected behavior
  }
}

// Client code breaks
function resizeRectangle(rect: Rectangle) {
  rect.setWidth(5);
  rect.setHeight(10);
  // Expected: 50, but Square returns 100!
  console.log(rect.getArea());
}
```

**Refactored**:
```typescript
// GOOD: Separate abstractions
interface Shape {
  getArea(): number;
}

class Rectangle implements Shape {
  constructor(private width: number, private height: number) {}
  getArea(): number { return this.width * this.height; }
}

class Square implements Shape {
  constructor(private side: number) {}
  getArea(): number { return this.side * this.side; }
}
```

### Pattern 2: Empty Implementations

**Indicators**:
- Methods that throw NotImplementedException
- Methods with empty bodies in subclasses
- Subclasses that override methods to do nothing

---

## Interface Segregation Principle (ISP) Violations

### Pattern 1: Fat Interfaces

**Example Violation** (Go):
```go
// BAD: Interface too large
type Worker interface {
    Work()
    Eat()
    Sleep()
    TakeVacation()
    AttendMeeting()
    SubmitReport()
    RequestRaise()
}

// Robot can't eat, sleep, or take vacation
type Robot struct{}

func (r Robot) Work() { /* works */ }
func (r Robot) Eat() { panic("robots don't eat") }  // Forced to implement
func (r Robot) Sleep() { panic("robots don't sleep") }
// ... more unused implementations
```

**Refactored**:
```go
// GOOD: Segregated interfaces
type Worker interface {
    Work()
}

type Eater interface {
    Eat()
}

type Employee interface {
    Worker
    Eater
    TakeVacation()
}

type Robot struct{}
func (r Robot) Work() { /* works */ }

type Human struct{}
func (h Human) Work() { /* works */ }
func (h Human) Eat() { /* eats */ }
func (h Human) TakeVacation() { /* vacations */ }
```

---

## Dependency Inversion Principle (DIP) Violations

### Pattern 1: Direct Instantiation

**Example Violation** (PHP):
```php
// BAD: High-level depends on low-level
class OrderService {
    private MySQLDatabase $db;
    private SMTPMailer $mailer;

    public function __construct() {
        $this->db = new MySQLDatabase('localhost', 'orders');
        $this->mailer = new SMTPMailer('smtp.example.com');
    }

    public function placeOrder(Order $order): void {
        $this->db->insert('orders', $order->toArray());
        $this->mailer->send($order->customerEmail, 'Order Placed');
    }
}
```

**Refactored**:
```php
// GOOD: Depend on abstractions
interface Database {
    public function insert(string $table, array $data): void;
}

interface Mailer {
    public function send(string $to, string $subject): void;
}

class OrderService {
    public function __construct(
        private Database $db,
        private Mailer $mailer
    ) {}

    public function placeOrder(Order $order): void {
        $this->db->insert('orders', $order->toArray());
        $this->mailer->send($order->customerEmail, 'Order Placed');
    }
}
```

---

## Detection Scripts

### Find Large Files (Bash)
```bash
find . -name "*.ts" -o -name "*.java" -o -name "*.py" | \
  xargs wc -l | sort -rn | head -20
```

### Find Classes with Many Methods (grep)
```bash
grep -r "class " --include="*.ts" -A 100 | \
  grep -E "^\s+(public|private|protected)?\s*\w+\s*\(" | wc -l
```

### Find Type Switches
```bash
grep -rn "switch.*type\|if.*instanceof\|if.*typeof" --include="*.ts"
```
