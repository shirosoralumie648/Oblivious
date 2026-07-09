package relay

import "oblivious/server/internal/relay/types"

func testPricingStore() *PricingStore {
	store := NewPricingStoreWithDefaults()
	store.SetPrice("gpt-4o", types.APITypeChat, types.DimTotalTokens, 0.001)
	store.SetPrice("gpt-4o-mini", types.APITypeChat, types.DimTotalTokens, 0.0001)
	return store
}

func testBillingHook() *BillingHook {
	return NewBillingHook(testPricingStore(), nil)
}
