package marketplace

import "math"

type MarketplaceRevenueTierDisclosure struct {
	CurrentTier                             string  `json:"currentTier"`
	Label                                   string  `json:"label"`
	MonthlySalesAmount                      float64 `json:"monthlySalesAmount"`
	PlatformFeeAmount                       float64 `json:"platformFeeAmount"`
	PublisherNetAmount                      float64 `json:"publisherNetAmount"`
	PlatformFeePercent                      float64 `json:"platformFeePercent"`
	PublisherSharePercent                   float64 `json:"publisherSharePercent"`
	EffectivePlatformFeePercent             float64 `json:"effectivePlatformFeePercent"`
	NextTierAt                              float64 `json:"nextTierAt,omitempty"`
	SalesToNextTier                         float64 `json:"salesToNextTier,omitempty"`
	EstimatedPublisherNetAtNextTier         float64 `json:"estimatedPublisherNetAtNextTier,omitempty"`
	EstimatedPublisherNetIncreaseAtNextTier float64 `json:"estimatedPublisherNetIncreaseAtNextTier,omitempty"`
}

type marketplaceRevenueTierSegment struct {
	name          string
	label         string
	minimumAmount float64
	feePercent    float64
}

var marketplaceRevenueTierSegments = []marketplaceRevenueTierSegment{
	{name: "tier_1", label: "Tier 1", minimumAmount: 0, feePercent: 30},
	{name: "tier_2", label: "Tier 2", minimumAmount: 1000, feePercent: 20},
	{name: "tier_3", label: "Tier 3", minimumAmount: 10000, feePercent: 15},
	{name: "tier_4", label: "Tier 4", minimumAmount: 100000, feePercent: 10},
}

func marketplaceRevenueTierDisclosure(monthlySalesAmount float64) MarketplaceRevenueTierDisclosure {
	sales := roundAmount(math.Max(monthlySalesAmount, 0))
	current := marketplaceRevenueTierSegments[0]
	currentIndex := 0
	for i, tier := range marketplaceRevenueTierSegments {
		if sales >= tier.minimumAmount {
			current = tier
			currentIndex = i
		}
	}

	platformFee := segmentedMarketplacePlatformFee(sales)
	publisherNet := roundAmount(sales - platformFee)
	disclosure := MarketplaceRevenueTierDisclosure{
		CurrentTier:                 current.name,
		Label:                       current.label,
		MonthlySalesAmount:          sales,
		PlatformFeeAmount:           platformFee,
		PublisherNetAmount:          publisherNet,
		PlatformFeePercent:          current.feePercent,
		PublisherSharePercent:       100 - current.feePercent,
		EffectivePlatformFeePercent: effectiveMarketplaceFeePercent(platformFee, sales),
	}

	if currentIndex+1 < len(marketplaceRevenueTierSegments) {
		nextTier := marketplaceRevenueTierSegments[currentIndex+1]
		disclosure.NextTierAt = nextTier.minimumAmount
		disclosure.SalesToNextTier = roundAmount(nextTier.minimumAmount - sales)
		if disclosure.SalesToNextTier < 0 {
			disclosure.SalesToNextTier = 0
		}
		disclosure.EstimatedPublisherNetAtNextTier = roundAmount(nextTier.minimumAmount - segmentedMarketplacePlatformFee(nextTier.minimumAmount))
		disclosure.EstimatedPublisherNetIncreaseAtNextTier = roundAmount(disclosure.EstimatedPublisherNetAtNextTier - publisherNet)
		if disclosure.EstimatedPublisherNetIncreaseAtNextTier < 0 {
			disclosure.EstimatedPublisherNetIncreaseAtNextTier = 0
		}
	}

	return disclosure
}

func segmentedMarketplacePlatformFee(amount float64) float64 {
	if amount <= 0 {
		return 0
	}
	total := 0.0
	for i, tier := range marketplaceRevenueTierSegments {
		nextMinimum := math.Inf(1)
		if i+1 < len(marketplaceRevenueTierSegments) {
			nextMinimum = marketplaceRevenueTierSegments[i+1].minimumAmount
		}
		if amount <= tier.minimumAmount {
			break
		}
		segmentAmount := math.Min(amount, nextMinimum) - tier.minimumAmount
		if segmentAmount > 0 {
			total += segmentAmount * tier.feePercent / 100
		}
		if amount < nextMinimum {
			break
		}
	}
	return roundAmount(total)
}

func effectiveMarketplaceFeePercent(platformFee float64, sales float64) float64 {
	if sales <= 0 {
		return 0
	}
	return roundAmount(platformFee * 100 / sales)
}
