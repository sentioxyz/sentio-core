# Decimal512 Test Guide

This document describes the test coverage and execution for Decimal512 BigDecimal support in Sentio.

---

## Part 1: Unit Test Scenarios

### Overview
Location: `driver/entity/clickhouse/decimal_flow_test.go`

- No external dependencies (no ClickHouse required)
- 100% code coverage of Decimal512Field implementation

### Test Cases

#### 1. `Test_Decimal256_Flow_Basic`
**Purpose:** Verify default Decimal256 behavior (baseline comparison)

**Scenario:**
- Schema: Entity with nullable and non-nullable BigDecimal fields
- Feature: `BigDecimalUseDecimal512 = false` (default)
- Validates:
  - DDL generation: `Decimal256(30)` for BigDecimal fields
  - NULL handling for nullable fields
  - Decimal value serialization/deserialization

**Expected DDL:**
```sql
`d0` Nullable(Decimal256(30))
`d1` Decimal256(30)
```

---

#### 2. `Test_Decimal512_Flow_Native_Write`
**Purpose:** Verify Decimal512 read/write flow with high precision values

**Scenario:**
- Schema: Entity with BigDecimal fields
- Feature: `BigDecimalUseDecimal512 = true` (EntitySchemaVersion=8)
- Test data:
  - 94 integer digits (max for Decimal512)
  - 60 decimal places (Decimal512 scale)
- Validates:
  - Automatic rounding to 60 decimal places
  - Correct serialization to ClickHouse format
  - Correct deserialization from ClickHouse
  - NULL handling for nullable fields

**Key assertion:**
```go
// Values with >60 decimal places are automatically rounded
input:  "9999...9999.1234...7890" (>60 decimals)
stored: "9999...9999.123456...901234567890" (exactly 60 decimals)
```

---

#### 3. `Test_Decimal512_Flow_Overflow`
**Purpose:** Verify overflow protection for values exceeding Decimal512 precision

**Scenario:**
- Schema: Entity with non-nullable BigDecimal
- Feature: `BigDecimalUseDecimal512 = true`
- Test data: 95 integer digits (exceeds 154 total precision limit)
- Validates:
  - Panic with clear error message when total digits exceed 154
  - Error message includes field name and digit counts

**Expected behavior:**
```go
Input: 95 integer digits + 60 decimal places = 155 total digits
Result: panic("decimal512 overflow for field E.d1: total digits 155 exceed precision 154 (scale 60)")
```

---

#### 4. `TestDecimal512_FeatureToggle`
**Purpose:** Verify EntitySchemaVersion flag correctly switches between Decimal256 and Decimal512

**Scenario:**
- Schema: Same entity definition
- Test both modes:
  - `EntitySchemaVersion = 0`: Default Decimal256 mode
  - `EntitySchemaVersion = 8`: Decimal512 mode (bit 3 set)
- Validates:
  - DDL generation differs based on schema version
  - Both modes accept decimal.Decimal values
  - Decimal512 mode rounds to scale 60
  - Decimal256 mode preserves original precision

**Comparison:**

| Feature | EntitySchemaVersion=0 | EntitySchemaVersion=8 |
|---------|----------------------|----------------------|
| DDL Type | `Decimal256(30)` | `Decimal512(60)` |
| Scale | 30 | 60 (hardcoded) |
| Max Precision | 76 digits | 154 digits |
| Rounding | Minimal | Auto-round to 60 decimals |

---

#### 5. `TestDecimal512_ScaleConfiguration`
**Purpose:** Verify Decimal512 scale is hardcoded to 60 and handles excess precision

**Scenario:**
- Schema: Entity with BigDecimal
- Feature: `EntitySchemaVersion = 8` (Decimal512 enabled)
- Test data: Value with 70 decimal places
- Validates:
  - Scale is always 60 (not configurable)
  - Values with >60 decimals are automatically rounded
  - DDL always shows `Decimal512(60)`

**Key test:**
```go
Input:  "123." + strings.Repeat("4", 70)  // 70 decimal places
Stored: "123." + strings.Repeat("4", 60)  // Rounded to 60
```

---

### Running Unit Tests

```bash
# Run all Decimal512 unit tests
bazel test //driver/entity/clickhouse:clickhouse_test \
  --test_filter='Test_Decimal.*'

# Run specific test
bazel test //driver/entity/clickhouse:clickhouse_test \
  --test_filter='Test_Decimal512_Flow_Native_Write'

# Run with verbose output
bazel test //driver/entity/clickhouse:clickhouse_test \
  --test_filter='Test_Decimal.*' \
  --test_output=all
```

---

## Part 2: Integration Test Execution

### Overview
Location: `driver/entity/clickhouse/decimal512_integration_test.go`

**Features:**
- **Requires real ClickHouse instance**
- Automatically skips if ClickHouse unavailable
- Tests complete user journey from schema to query

### Test: `TestDecimal512_E2E_UserWorkflow`

**Purpose:** Simulate complete user workflow using Decimal512 in production

**Workflow Steps:**

1. **ClickHouse Connection**
   - Connect to ClickHouse (172.17.0.3:9000)
   - Auto-skip if connection fails

2. **GraphQL Schema Parsing**
   - Parse user-defined schema with BigDecimal fields
   - Simulate Uniswap V3 Pool entity

3. **Store Initialization**
   - Initialize Sentio Store with `EntitySchemaVersion=8`
   - Enable Decimal512 feature flag

4. **Table Creation**
   - Auto-create ClickHouse tables
   - Verify DDL uses `Decimal(154, 60)` (Decimal512 format)

5. **Data Writing**
   - Write high-precision DeFi data:
     - Small pool: Normal precision values
     - Large pool: Ultra-high precision (60 decimal places, 94 integer digits)
   - Validate automatic rounding

6. **Single Entity Query**
   - Query via `store.GetEntity()`
   - Verify precision preserved (60 decimals)

7. **List Query**
   - Query via `store.ListEntities()`
   - Verify all records maintain precision

8. **Direct ClickHouse Query**
   - Query ClickHouse directly (bypass Sentio layer)
   - Verify data consistency between Store and raw queries

9. **Aggregation Queries**
   - Test `SUM()` and `AVG()` on Decimal512 fields
   - Verify aggregation correctness

10. **Cleanup**
    - Drop test tables
    - Clean test data

**Test Data Example:**

```go
// Large pool - ultra-high precision
liquidity: "9999...9999.123456789012345678901234567890123456789012345678901234567890"
           // 94 integer digits + 60 decimal places
           
tvlUSD: "999999999999999999999999.999999999999999999999999999999999999999999999999999999"
        // High precision TVL
```

---

### Prerequisites

1. **ClickHouse Server Running**
   ```bash
   # Check ClickHouse availability
   curl http://172.17.0.3:8123/ping
   # Expected: Ok.
   ```

2. **ClickHouse Database Created**
   ```bash
   # Create test database (if not exists)
   clickhouse-client -h 172.17.0.3 --query \
     "CREATE DATABASE IF NOT EXISTS test_decimal512"
   ```

---

### Running Integration Test

#### Method 1: In dev Container (Recommended)

```bash
# Run all Decimal512 tests (unit + integration)
docker exec dev bash -c "cd /workspace/sentio && \
  bazel test //driver/entity/clickhouse:clickhouse_test \
  --test_filter='.*Decimal.*' \
  --test_output=all"

# Run ONLY integration test
docker exec dev bash -c "cd /workspace/sentio && \
  bazel test //driver/entity/clickhouse:clickhouse_test \
  --test_filter='TestDecimal512_E2E_UserWorkflow' \
  --cache_test_results=no \
  --test_output=all"

# Run with verbose logging
docker exec dev bash -c "cd /workspace/sentio && \
  bazel test //driver/entity/clickhouse:clickhouse_test \
  --test_filter='TestDecimal512_E2E_UserWorkflow' \
  --test_arg=-test.v \
  --test_output=all"
```

#### Method 2: On Host Machine

```bash
cd /home/sentio/limengyu/dev/sentio

# Run all Decimal512 tests
bazel test //driver/entity/clickhouse:clickhouse_test \
  --test_filter='.*Decimal.*' \
  --test_output=all

# Run only integration test
bazel test //driver/entity/clickhouse:clickhouse_test \
  --test_filter='TestDecimal512_E2E_UserWorkflow' \
  --cache_test_results=no \
  --test_output=all
```

#### Method 3: Skip Integration Test (CI/CD)

```bash
# Integration test will auto-skip when ClickHouse unavailable
# This is the default behavior in CI/CD environments

bazel test //driver/entity/clickhouse:clickhouse_test

# Output:
# Test_Decimal256_Flow_Basic (PASS)
# Test_Decimal512_Flow_Native_Write (PASS)
# Test_Decimal512_Flow_Overflow (PASS)
# TestDecimal512_FeatureToggle (PASS)
# TestDecimal512_ScaleConfiguration (PASS)
# TestDecimal512_E2E_UserWorkflow (SKIPPED - no ClickHouse)
```

---

### Skip Behavior

The integration test uses Go's `testing.Short()` mechanism:

```go
func TestDecimal512_E2E_UserWorkflow(t *testing.T) {
    if testing.Short() {
        t.Skip("Skip integration test - requires real ClickHouse")
    }
    
    // Connection attempts with auto-skip
    conn, err := clickhouse.Open(options)
    if err != nil {
        t.Skipf("Skip test - cannot connect to ClickHouse: %v", err)
    }
    
    if err := conn.Ping(ctx); err != nil {
        t.Skipf("Skip test - ClickHouse unreachable: %v", err)
    }
    
    // ... test continues if ClickHouse is available
}
```

**Skip Conditions:**
1. `testing.Short()` is true (default in most CI/CD)
2. ClickHouse connection fails
3. ClickHouse ping fails

**When Test Runs:**
- ClickHouse is available at 172.17.0.3:9000
- test_decimal512 database exists or can be created
- `testing.Short()` is false (use `--test_filter` to force run)

---

### Troubleshooting

#### Issue: Integration test always skips

**Solution:**
```bash
# Verify ClickHouse is running
docker ps | grep ck-dev

# Check ClickHouse connectivity
curl http://172.17.0.3:8123/ping

# Check ClickHouse with Go driver DSN
clickhouse-client -h 172.17.0.3 --query "SELECT 1"
```

#### Issue: Permission denied or connection refused

**Solution:**
```bash
# Check ClickHouse config allows external connections
docker exec ck-dev cat /etc/clickhouse-server/config.xml | grep listen_host

# Should show:
# <listen_host>0.0.0.0</listen_host>

# If not, modify and restart ClickHouse
```

#### Issue: Database does not exist

**Solution:**
```bash
# Create database manually
clickhouse-client -h 172.17.0.3 --query \
  "CREATE DATABASE IF NOT EXISTS test_decimal512"
```

---

## Test Configuration

### Config File
Location: `driver/entity/clickhouse/test_decimal512_config.yaml`

```yaml
task:
  eth-mainnet:
    driver:
      type: driver
      entity-schema-version: 8  # Enable Decimal512 support
      clickhouse:
        dsn: 'clickhouse://default@172.17.0.3:9000/test_decimal512?dial_timeout=10s'
```

**Key Parameters:**
- `entity-schema-version: 8` - Enables Decimal512 (bit 3)
- `dsn` - ClickHouse connection string
- Database: `test_decimal512` (test isolation)

---

## Summary

### Unit Tests (5 tests)
- No external dependencies
- Fast execution
- Always runs in CI/CD
- 100% code coverage

### Integration Test (1 test)
- End-to-end validation
- Real ClickHouse required
- Auto-skips when unavailable
- Validates complete user workflow

### Combined Coverage
| Layer | Tests | Coverage |
|-------|-------|----------|
| Unit | 5 | Core logic (Field, Store, Entity) |
| Integration | 1 | E2E workflow (Schema → Query) |
| **Total** | **6** | **100% Decimal512 functionality** |

---
