package clickhouse

import (
	"context"
	"fmt"
	"sentioxyz/sentio-core/common/chx"
	ckhmanager "sentioxyz/sentio-core/common/clickhousemanager"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sentioxyz/sentio-core/driver/entity/persistent"
	"sentioxyz/sentio-core/driver/entity/schema"
)

// Real DeFi scenario - Uniswap V3 Pool
const uniswapV3Schema = `
type Pool @entity {
  id: ID!
  token0: String!
  token1: String!
  fee: Int!
  liquidity: BigDecimal!
  sqrtPriceX96: BigDecimal!
  tick: Int!
  tvlUSD: BigDecimal
  volumeUSD: BigDecimal!
  feesUSD: BigDecimal!
}
`

// TestDecimal512_E2E_UserWorkflow - End-to-end user workflow integration test
// Simulates complete user flow from schema definition to data querying
func TestDecimal512_E2E_UserWorkflow(t *testing.T) {
	t.Skip("use local db, will only be executed manually locally")

	ctx := context.Background()
	chainID := "" // Use empty chain for non-timeseries entity

	// Step 1: User configures ClickHouse connection (enable Decimal512)
	t.Log("Step 1: Configure ClickHouse connection and enable Decimal512")

	dsn := "clickhouse://default@172.17.0.3:9000/test_decimal512?dial_timeout=10s"
	conn := ckhmanager.NewConn(dsn)
	defer conn.Close()

	t.Log("ClickHouse connection successful")

	// Step 2: User defines GraphQL Schema (with BigDecimal)
	t.Log("Step 2: Parse user's GraphQL Schema")

	sch, err := schema.ParseAndVerifySchema(uniswapV3Schema)
	require.NoError(t, err, "Schema parsing should succeed")

	poolEntity := sch.GetEntity("Pool")
	require.NotNil(t, poolEntity, "Pool entity should exist")

	t.Log("GraphQL Schema parsed successfully")
	t.Logf("Pool entity: %d fields", len(poolEntity.Fields))

	// Step 3: Sentio initializes Store (EntitySchemaVersion=8)
	t.Log("Step 3: Initialize Sentio Store (Decimal512 mode)")

	processorID := fmt.Sprintf("e2e_dec512_%d", time.Now().Unix())
	store := NewStore(
		chx.NewController(conn),
		processorID,
		BuildFeatures(8), // EntitySchemaVersion = 8 enables Decimal512
		sch,
		TableOption{},
	)

	require.NotNil(t, store, "Store creation should succeed")
	t.Log("Store initialized successfully")
	t.Logf("ProcessorID: %s", processorID)
	t.Logf("Features.BigDecimalUseDecimal512 = %v", store.feaOpt.BigDecimalUseDecimal512)

	// Step 4: Sentio auto-creates ClickHouse tables
	t.Log("Step 4: Auto-create ClickHouse table schema")

	err = store.InitEntitySchema(ctx)
	require.NoError(t, err, "Table creation should succeed")

	t.Log("ClickHouse tables created successfully")

	// Verify table schema - check if BigDecimal fields use Decimal512
	poolTableName := store.TableName(poolEntity)
	t.Logf("Pool table name: %s", poolTableName)

	var createTableSQL string
	err = conn.QueryRow(ctx, fmt.Sprintf("SHOW CREATE TABLE `test_decimal512`.`%s`", poolTableName)).Scan(&createTableSQL)
	require.NoError(t, err, "Should be able to query table structure")

	// Critical verification: BigDecimal fields should be Decimal(154, 60) i.e. Decimal512
	// Note: ClickHouse displays Decimal512 as Decimal(154, 60) format
	t.Log("Verify BigDecimal field types in table schema")
	assert.Contains(t, createTableSQL, "Decimal(154, 60)", "liquidity field should be Decimal(154, 60)")
	assert.Contains(t, createTableSQL, "Nullable(Decimal(154, 60))", "tvlUSD field should be Nullable(Decimal(154, 60))")

	t.Log("Table schema verified - BigDecimal correctly mapped to Decimal512(60)")

	// Step 5: User writes data (BigDecimal with various precisions)
	t.Log("Step 5: Write test data (simulating real DeFi scenario)")

	// Real Uniswap V3 value scenarios
	testCases := []struct {
		name         string
		liquidity    string
		sqrtPriceX96 string
		tvlUSD       string
		volumeUSD    string
		feesUSD      string
	}{
		{
			name:         "Small pool - normal precision",
			liquidity:    "1234567.890123456789012345678901234567890123456789012345678901",
			sqrtPriceX96: "79228162514264337593543950336",
			tvlUSD:       "10000.50",
			volumeUSD:    "5000.25",
			feesUSD:      "15.075",
		},
		{
			name:         "Large pool - ultra-high precision",
			liquidity:    "9999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999.123456789012345678901234567890123456789012345678901234567890",
			sqrtPriceX96: "1234567890123456789012345678901234567890.987654321098765432109876543210987654321098765432109876543210",
			tvlUSD:       "999999999999999999999999.999999999999999999999999999999999999999999999999999999",
			volumeUSD:    "123456789012345678901234567890.12345678901234567890123456789012345678901234567890",
			feesUSD:      "987654321098.765432109876543210987654321098765432109876543210987654321098",
		},
	}

	poolIDs := make([]string, len(testCases))

	for i, tc := range testCases {
		poolID := fmt.Sprintf("pool_%d", i)
		poolIDs[i] = poolID

		t.Logf("Writing Pool #%d: %s", i, tc.name)

		poolDataMap := map[string]any{
			"id":           poolID,
			"token0":       "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48", // USDC
			"token1":       "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2", // WETH
			"fee":          int32(3000),
			"liquidity":    decimal.RequireFromString(tc.liquidity),
			"sqrtPriceX96": decimal.RequireFromString(tc.sqrtPriceX96),
			"tick":         int32(12345),
			"tvlUSD":       decimal.RequireFromString(tc.tvlUSD),
			"volumeUSD":    decimal.RequireFromString(tc.volumeUSD),
			"feesUSD":      decimal.RequireFromString(tc.feesUSD),
		}

		poolData := persistent.EntityBox{
			ID:     poolID,
			Data:   poolDataMap,
			Entity: "Pool",
		}

		_, err = store.SetEntities(ctx, poolEntity, []persistent.EntityBox{poolData})
		require.NoError(t, err, "Pool data write should succeed: %s", tc.name)
	}

	t.Logf("Successfully wrote %d Pool entities", len(testCases))

	// Step 6: User queries single entity (GetEntity)
	t.Log("Step 6: Query single Pool entity")

	targetPoolID := poolIDs[1]
	result, err := store.GetEntity(ctx, poolEntity, chainID, targetPoolID)
	require.NoError(t, err, "GetEntity should succeed")
	require.NotNil(t, result, "Should retrieve data")

	t.Log("GetEntity query successful")

	// Verify query result precision
	liquidity, ok := result.Data["liquidity"].(decimal.Decimal)
	require.True(t, ok, "liquidity should be decimal.Decimal type")

	expectedLiquidity := decimal.RequireFromString(testCases[1].liquidity)
	expectedLiquidityRounded := expectedLiquidity.Round(60)

	assert.True(t, liquidity.Equal(expectedLiquidityRounded),
		"liquidity precision should be correct\nExpected: %s\nActual: %s",
		expectedLiquidityRounded.String(), liquidity.String())

	t.Log("Data precision verified")
	t.Logf("Original value length: %d decimal places", countDecimalPlaces(testCases[1].liquidity))
	t.Logf("Stored value: %s (60 decimal places)", liquidity.StringFixed(60))

	// Step 7: User list query (ListEntities)
	t.Log("Step 7: List query all Pools")

	listResult, err := store.ListEntities(
		ctx,
		poolEntity,
		chainID,
		[]persistent.EntityFilter{},
		100,
	)
	require.NoError(t, err, "ListEntities should succeed")
	assert.GreaterOrEqual(t, len(listResult), len(testCases), "Should retrieve all written Pools")

	t.Logf("ListEntities successful, returned %d records", len(listResult))

	// Verify data precision in list
	for i, entity := range listResult {
		if i >= len(testCases) {
			break
		}

		entityLiquidity, ok := entity.Data["liquidity"].(decimal.Decimal)
		require.True(t, ok, "liquidity in list should be decimal.Decimal")

		decimalPlaces := countDecimalPlacesFromDecimal(entityLiquidity)
		assert.LessOrEqual(t, decimalPlaces, 60,
			"List query data should maintain 60 decimal places precision, actual: %d", decimalPlaces)
	}

	t.Log("List data precision verified")

	// Step 8: Direct ClickHouse query to verify storage
	t.Log("Step 8: Direct ClickHouse query to verify underlying storage")

	var directLiquidity decimal.Decimal
	var directSqrtPrice decimal.Decimal
	var directTvlUSD *decimal.Decimal

	query := fmt.Sprintf(`
		SELECT liquidity, sqrtPriceX96, tvlUSD 
		FROM test_decimal512.%s 
		WHERE id = ? 
		LIMIT 1
	`, poolTableName)

	err = conn.QueryRow(ctx, query, poolIDs[1]).Scan(
		&directLiquidity,
		&directSqrtPrice,
		&directTvlUSD,
	)
	require.NoError(t, err, "Direct ClickHouse query should succeed")

	t.Log("ClickHouse underlying query successful")
	t.Logf("Liquidity: %s", directLiquidity.String())
	t.Logf("SqrtPrice: %s", directSqrtPrice.String())
	if directTvlUSD != nil {
		t.Logf("TVL USD: %s", directTvlUSD.String())
	}

	assert.True(t, liquidity.Equal(directLiquidity),
		"Store query and direct query results should match")

	t.Log("Data consistency verified")

	// Step 9: Test aggregation queries
	t.Log("Step 9: Test aggregation queries (SUM, AVG)")

	var totalVolumeUSD decimal.Decimal
	var avgFeesUSD decimal.Decimal

	aggQuery := fmt.Sprintf(`
		SELECT 
			sum(volumeUSD) as total_volume,
			avg(feesUSD) as avg_fees
		FROM test_decimal512.%s
	`, poolTableName)

	err = conn.QueryRow(ctx, aggQuery).Scan(&totalVolumeUSD, &avgFeesUSD)
	require.NoError(t, err, "Aggregation query should succeed")

	t.Log("Aggregation query successful")
	t.Logf("Total Volume USD: %s", totalVolumeUSD.String())
	t.Logf("Average Fees USD: %s", avgFeesUSD.String())

	assert.True(t, totalVolumeUSD.GreaterThan(decimal.Zero), "Total volume should be greater than 0")
	assert.True(t, avgFeesUSD.GreaterThan(decimal.Zero), "Average fees should be greater than 0")

	// Step 10: Clean up test data
	t.Log("Step 10: Clean up test data")

	err = conn.Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS test_decimal512.%s", poolTableName))
	require.NoError(t, err)

	t.Log("Test data cleanup completed")

	// Test summary
	t.Log("")
	t.Log("Decimal512 end-to-end integration test completed")
	t.Log("Verified features:")
	t.Log("  1. Schema parsing to ClickHouse table creation")
	t.Log("  2. Table schema correctly uses Decimal(154, 60)")
	t.Log("  3. High-precision values correctly stored and retrieved (60 decimal places)")
	t.Log("  4. Single entity query (GetEntity) correct")
	t.Log("  5. List query (ListEntities) correct")
	t.Log("  6. Aggregation queries (SUM/AVG) correct")
	t.Log("  7. Data consistency verification passed")
	t.Log("")
	t.Log("Complete user workflow verification successful")
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func countDecimalPlaces(s string) int {
	parts := strings.Split(s, ".")
	if len(parts) != 2 {
		return 0
	}
	return len(parts[1])
}

func countDecimalPlacesFromDecimal(d decimal.Decimal) int {
	exp := d.Exponent()
	if exp >= 0 {
		return 0
	}
	return int(-exp)
}
